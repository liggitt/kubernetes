/*
Copyright 2020 The Kubernetes Authors.

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

package scheme

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-scheduler/config/v1alpha2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/utils/pointer"
)

func TestCodecsDecodePluginConfig(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
		wantErr      string
		wantProfiles []config.KubeSchedulerProfile
	}{
		{
			name: "v1alpha2 all plugin args in default profile",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: InterPodAffinity
    args:
      hardPodAffinityWeight: 5
  - name: NodeLabel
    args:
      presentLabels: ["foo"]
  - name: NodeResourcesFit
    args:
      ignoredResources: ["foo"]
  - name: RequestedToCapacityRatio
    args:
      shape:
      - utilization: 1
  - name: PodTopologySpread
    args:
      defaultConstraints:
      - maxSkew: 1
        topologyKey: zone
        whenUnsatisfiable: ScheduleAnyway
  - name: ServiceAffinity
    args:
      affinityLabels: ["bar"]
`),
			wantProfiles: []config.KubeSchedulerProfile{
				{
					SchedulerName: "default-scheduler",
					PluginConfig: []config.PluginConfig{
						{
							Name: "InterPodAffinity",
							Args: &config.InterPodAffinityArgs{HardPodAffinityWeight: 5},
						},
						{
							Name: "NodeLabel",
							Args: &config.NodeLabelArgs{PresentLabels: []string{"foo"}},
						},
						{
							Name: "NodeResourcesFit",
							Args: &config.NodeResourcesFitArgs{IgnoredResources: []string{"foo"}},
						},
						{
							Name: "RequestedToCapacityRatio",
							Args: &config.RequestedToCapacityRatioArgs{
								Shape: []config.UtilizationShapePoint{{Utilization: 1}},
							},
						},
						{
							Name: "PodTopologySpread",
							Args: &config.PodTopologySpreadArgs{
								DefaultConstraints: []v1.TopologySpreadConstraint{
									{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: v1.ScheduleAnyway},
								},
							},
						},
						{
							Name: "ServiceAffinity",
							Args: &config.ServiceAffinityArgs{
								AffinityLabels: []string{"bar"},
							},
						},
					},
				},
			},
		},
		{
			name: "v1alpha2 plugins can include version and kind",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: NodeLabel
    args:
      apiVersion: kubescheduler.config.k8s.io/v1alpha2
      kind: NodeLabelArgs
      presentLabels: ["bars"]
`),
			wantProfiles: []config.KubeSchedulerProfile{
				{
					SchedulerName: "default-scheduler",
					PluginConfig: []config.PluginConfig{
						{
							Name: "NodeLabel",
							Args: &config.NodeLabelArgs{PresentLabels: []string{"bars"}},
						},
					},
				},
			},
		},
		{
			name: "plugin group and kind should match the type",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: NodeLabel
    args:
      apiVersion: kubescheduler.config.k8s.io/v1alpha2
      kind: InterPodAffinityArgs
`),
			wantErr: "decoding .profiles[0].pluginConfig[0]: args for plugin NodeLabel were not of type NodeLabelArgs.kubescheduler.config.k8s.io, got InterPodAffinityArgs.kubescheduler.config.k8s.io",
		},
		{
			// TODO: do not replicate this case for v1beta1.
			name: "v1alpha2 case insensitive RequestedToCapacityRatioArgs",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: RequestedToCapacityRatio
    args:
      shape:
      - utilization: 1
        score: 2
      - Utilization: 3
        Score: 4
      resources:
      - name: Upper
        weight: 1
      - Name: lower
        weight: 2
`),
			wantProfiles: []config.KubeSchedulerProfile{
				{
					SchedulerName: "default-scheduler",
					PluginConfig: []config.PluginConfig{
						{
							Name: "RequestedToCapacityRatio",
							Args: &config.RequestedToCapacityRatioArgs{
								Shape: []config.UtilizationShapePoint{
									{Utilization: 1, Score: 2},
									{Utilization: 3, Score: 4},
								},
								Resources: []config.ResourceSpec{
									{Name: "Upper", Weight: 1},
									{Name: "lower", Weight: 2},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "out-of-tree plugin args",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: OutOfTreePlugin
    args:
      foo: bar
`),
			wantProfiles: []config.KubeSchedulerProfile{
				{
					SchedulerName: "default-scheduler",
					PluginConfig: []config.PluginConfig{
						{
							Name: "OutOfTreePlugin",
							Args: &runtime.Unknown{
								ContentType: "application/json",
								Raw:         []byte(`{"foo":"bar"}`),
							},
						},
					},
				},
			},
		},
		{
			name: "empty and no plugin args",
			data: []byte(`
apiVersion: kubescheduler.config.k8s.io/v1alpha2
kind: KubeSchedulerConfiguration
profiles:
- pluginConfig:
  - name: InterPodAffinity
    args:
  - name: NodeResourcesFit
  - name: OutOfTreePlugin
    args:
`),
			wantProfiles: []config.KubeSchedulerProfile{
				{
					SchedulerName: "default-scheduler",
					PluginConfig: []config.PluginConfig{
						{
							Name: "InterPodAffinity",
							// TODO(acondor): Set default values.
							Args: &config.InterPodAffinityArgs{},
						},
						{
							Name: "NodeResourcesFit",
							Args: &config.NodeResourcesFitArgs{},
						},
						{Name: "OutOfTreePlugin"},
					},
				},
			},
		},
	}
	decoder := Codecs.UniversalDecoder()
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			obj, gvk, err := decoder.Decode(tt.data, nil, nil)
			if err != nil {
				if tt.wantErr != err.Error() {
					t.Fatalf("got err %w, want %w", err, tt.wantErr)
				}
				return
			}
			if len(tt.wantErr) != 0 {
				t.Fatal("no error produced, wanted %w", tt.wantErr)
			}
			got, ok := obj.(*config.KubeSchedulerConfiguration)
			if !ok {
				t.Fatalf("decoded into %s, want %s", gvk, config.SchemeGroupVersion.WithKind("KubeSchedulerConfiguration"))
			}
			if diff := cmp.Diff(tt.wantProfiles, got.Profiles); diff != "" {
				t.Errorf("unexpected configuration (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestCodecsEncodePluginConfig(t *testing.T) {
	testCases := []struct {
		name    string
		obj     runtime.Object
		version schema.GroupVersion
		want    string
	}{
		{
			name:    "v1alpha2 in-tree and out-of-tree plugins",
			version: v1alpha2.SchemeGroupVersion,
			obj: &v1alpha2.KubeSchedulerConfiguration{
				Profiles: []v1alpha2.KubeSchedulerProfile{
					{
						PluginConfig: []v1alpha2.PluginConfig{
							{
								Name: "InterPodAffinity",
								Args: runtime.RawExtension{
									Object: &v1alpha2.InterPodAffinityArgs{
										HardPodAffinityWeight: pointer.Int32Ptr(5),
									},
								},
							},
							{
								Name: "OutOfTreePlugin",
								Args: runtime.RawExtension{
									Raw: []byte(`{"foo":"bar"}`),
								},
							},
						},
					},
				},
			},
			want: `apiVersion: kubescheduler.config.k8s.io/v1alpha2
bindTimeoutSeconds: null
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
extenders: null
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: null
  leaseDuration: 0s
  renewDeadline: 0s
  resourceLock: ""
  resourceName: ""
  resourceNamespace: ""
  retryPeriod: 0s
podInitialBackoffSeconds: null
podMaxBackoffSeconds: null
profiles:
- pluginConfig:
  - args:
      hardPodAffinityWeight: 5
    name: InterPodAffinity
  - args:
      foo: bar
    name: OutOfTreePlugin
`,
		},
		// Encoding from internal is not supported.
	}
	info, ok := runtime.SerializerInfoForMediaType(Codecs.SupportedMediaTypes(), runtime.ContentTypeYAML)
	if !ok {
		t.Fatalf("unable to locate encoder -- %q is not a supported media type", runtime.ContentTypeYAML)
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			encoder := Codecs.EncoderForVersion(info.Serializer, tt.version)
			var buf bytes.Buffer
			if err := encoder.Encode(tt.obj, &buf); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("unexpected encoded configuration:\n%s", diff)
			}
		})
	}
}
