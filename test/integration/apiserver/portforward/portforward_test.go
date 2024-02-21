/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package portforward

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/apimachinery/pkg/util/wait"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/kubernetes"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/kubectl/pkg/cmd/portforward"
	kubeletportforward "k8s.io/kubelet/pkg/cri/streaming/portforward"
	kastesting "k8s.io/kubernetes/cmd/kube-apiserver/app/testing"
	kubefeatures "k8s.io/kubernetes/pkg/features"

	"k8s.io/kubernetes/test/integration/framework"
)

func TestPortforward(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, kubefeatures.PortForwardWebsockets, true)()
	t.Setenv("KUBECTL_PORT_FORWARD_WEBSOCKETS", "true")

	var podName string
	var podUID types.UID
	backendServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("backend saw request: %v", req.URL.String())
		kubeletportforward.ServePortForward(
			w,
			req,
			&dummyPortForwarder{t: t},
			podName,
			podUID,
			&kubeletportforward.V4Options{},
			wait.ForeverTestTimeout, // idle timeout
			remotecommand.DefaultStreamCreationTimeout, // stream creation timeout
			[]string{kubeletportforward.ProtocolV1Name},
		)
	}))
	defer backendServer.Close()
	backendURL, _ := url.Parse(backendServer.URL)
	backendHost := backendURL.Hostname()
	backendPort, _ := strconv.Atoi(backendURL.Port())

	etcd := framework.SharedEtcd()
	server := kastesting.StartTestServerOrDie(t, nil, []string{"--disable-admission-plugins=ServiceAccount"}, etcd)
	defer server.TearDownFn()

	adminClient, err := kubernetes.NewForConfig(server.ClientConfig)
	require.NoError(t, err)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "mynode"},
		Status: corev1.NodeStatus{
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: int32(backendPort)}},
			Addresses:       []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: backendHost}},
		},
	}
	if _, err := adminClient.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "mypod"},
		Spec: corev1.PodSpec{
			NodeName:   "mynode",
			Containers: []corev1.Container{{Name: "test", Image: "test"}},
		},
	}
	if _, err := adminClient.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := adminClient.CoreV1().Pods("default").Patch(context.Background(), "mypod", types.MergePatchType, []byte(`{"status":{"phase":"Running"}}`), metav1.PatchOptions{}, "status"); err != nil {
		t.Fatal(err)
	}

	streams, _, out, errOut := genericiooptions.NewTestIOStreams()
	portForwardOptions := portforward.NewDefaultPortForwardOptions(streams)
	portForwardOptions.Namespace = "default"
	portForwardOptions.PodName = "mypod"
	portForwardOptions.RESTClient = adminClient.CoreV1().RESTClient()
	portForwardOptions.Config = server.ClientConfig
	portForwardOptions.PodClient = adminClient.CoreV1()
	portForwardOptions.Address = []string{"127.0.0.1"}
	portForwardOptions.Ports = []string{"8000"} // TODO: find free port
	portForwardOptions.StopChannel = make(chan struct{}, 1)
	portForwardOptions.ReadyChannel = make(chan struct{})

	if err := portForwardOptions.Validate(); err != nil {
		t.Fatal(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := portForwardOptions.RunPortForward(); err != nil {
			t.Error(err)
		}
	}()

	t.Log("waiting for port forward to be ready")
	select {
	case <-portForwardOptions.ReadyChannel:
		t.Log("port forward was ready")
	case <-time.After(wait.ForeverTestTimeout):
		t.Error("port forward was never ready")
	}

	timeoutContext, cleanupTimeoutContext := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	defer cleanupTimeoutContext()
	testReq, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	testReq = testReq.WithContext(timeoutContext)
	testResp, err := http.DefaultClient.Do(testReq)
	if err != nil {
		t.Error(err)
	} else {
		t.Log(testResp.StatusCode)
		data, err := io.ReadAll(testResp.Body)
		if err != nil {
			t.Error(err)
		} else {
			t.Log("client saw reponse:", string(data))
		}
		if string(data) != "request to 8000 was ok" {
			t.Errorf("unexpected data")
		}
		if testResp.StatusCode != 200 {
			t.Error("expected success")
		}
	}

	close(portForwardOptions.StopChannel)

	wg.Wait()
	t.Logf("stdout: %s", out.String())
	t.Logf("stderr: %s", errOut.String())
}

type dummyPortForwarder struct {
	t *testing.T
}

func (d *dummyPortForwarder) PortForward(ctx context.Context, name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	d.t.Logf("handling port forward request for %d", port)

	req, err := http.ReadRequest(bufio.NewReader(stream))
	if err != nil {
		d.t.Logf("error reading request: %v", err)
		return err
	}
	d.t.Log(req.URL.String())
	defer req.Body.Close()

	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Body:       io.NopCloser(bytes.NewBufferString(fmt.Sprintf("request to %d was ok", port))),
	}
	resp.Write(stream)
	return stream.Close()
}
