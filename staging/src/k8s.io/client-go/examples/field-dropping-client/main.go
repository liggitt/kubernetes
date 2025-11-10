package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const unpersistableDroppedKey = "NO NO NO never persist this is a list of dropped fields"

func main() {
	config := getRESTConfig()

	simulateObjects(config)

	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	config.APIPath = "/api"
	config.NegotiatedSerializer = &detectingStrictNegotiatedWrapper{NegotiatedSerializer: scheme.Codecs.WithoutConversion()}

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		log.Fatal(err)
	}

	typedClient := gentype.NewClientWithList[*corev1.PodTemplate, *corev1.PodTemplateList](
		"podtemplates",
		restClient,
		scheme.ParameterCodec,
		"default",
		func() *corev1.PodTemplate { return &corev1.PodTemplate{} },
		func() *corev1.PodTemplateList { return &corev1.PodTemplateList{} },
	)

	list, err := typedClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range list.Items {
		observe("list", &item)
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				return typedClient.List(ctx, options)
			},
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				return typedClient.Watch(ctx, options)
			},
		},
		&corev1.PodTemplate{},
		0,
		nil,
	)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			observe("watch add", obj.(*corev1.PodTemplate))
		},
		UpdateFunc: func(old, new interface{}) {
			observe("watch update", new.(*corev1.PodTemplate))
		},
		DeleteFunc: func(obj interface{}) {
			if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = tombstone.Obj
			}
			observe("watch delete", obj.(*corev1.PodTemplate))
		},
	})
	fmt.Println("running informer...")
	defer fmt.Println("exiting")
	informer.RunWithContext(context.Background())
}

type detectingStrictNegotiatedWrapper struct {
	runtime.NegotiatedSerializer
}

func (s *detectingStrictNegotiatedWrapper) SupportedMediaTypes() []runtime.SerializerInfo {
	retval := s.NegotiatedSerializer.SupportedMediaTypes()
	for _, info := range retval {
		if info.MediaType == "application/json" {
			info.Serializer = &detectingStrictDeserializer{Serializer: info.StrictSerializer}
			info.StreamSerializer.Serializer = &detectingStrictDeserializer{Serializer: info.StreamSerializer.Serializer}
			return []runtime.SerializerInfo{info}
		}
	}
	return []runtime.SerializerInfo{}
}

type detectingStrictDeserializer struct {
	runtime.Serializer
}

var unknownFieldRegex = regexp.MustCompile(`^unknown field "(.*)"$`)

func getUnknownField(err error) (string, bool) {
	matches := unknownFieldRegex.FindStringSubmatch(err.Error())
	if len(matches) > 0 {
		return matches[1], true
	}
	return "", false
}

var itemUnknownFieldRegex = regexp.MustCompile(`^unknown field "items\[(\d+)\]\.(.*)"$`)

func getItemUnknownField(err error) (int, string, bool) {
	matches := itemUnknownFieldRegex.FindStringSubmatch(err.Error())
	if len(matches) > 0 {
		i, _ := strconv.Atoi(matches[1])
		return i, matches[2], true
	}
	return 0, "", false
}

func (t *detectingStrictDeserializer) Decode(data []byte, defaults *schema.GroupVersionKind, into runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	obj, gvk, err := t.Serializer.Decode(data, defaults, into)
	if strictError, isStrictError := runtime.AsStrictDecodingError(err); isStrictError && obj != nil {
		switch obj := obj.(type) {
		case *corev1.PodTemplateList:
			droppedByIndex := map[int][]string{}
			for _, err := range strictError.Errors() {
				if i, field, unknown := getItemUnknownField(err); unknown {
					droppedByIndex[i] = append(droppedByIndex[i], field)
				}
			}
			for i, dropped := range droppedByIndex {
				if i >= len(obj.Items) {
					continue
				}
				item := &obj.Items[i]
				if item.Annotations == nil {
					item.Annotations = map[string]string{}
				}
				item.Annotations[unpersistableDroppedKey] = strings.Join(dropped, ", ")
			}
		case *corev1.PodTemplate:
			dropped := []string{}
			for _, err := range strictError.Errors() {
				if field, unknown := getUnknownField(err); unknown {
					dropped = append(dropped, field)
				}
			}
			if len(dropped) > 0 {
				if obj.Annotations == nil {
					obj.Annotations = map[string]string{}
				}
				obj.Annotations[unpersistableDroppedKey] = strings.Join(dropped, ", ")
			}
		default:
			fmt.Printf("unknown type %T: %v\n", obj, strictError.Errors())
		}
		err = nil
	}
	return obj, gvk, err
}

func getRESTConfig() *rest.Config {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		kubeconfig = filepath.Join(homedir, ".kube", "kubeconfig")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func observe(operation string, deployment *corev1.PodTemplate) {
	warning, droppedUnknown := deployment.Annotations[unpersistableDroppedKey]
	if droppedUnknown {
		fmt.Println(operation, deployment.ResourceVersion, deployment.Name, warning)
	} else {
		fmt.Println(operation, deployment.ResourceVersion, deployment.Name, "ok")
	}
}

func simulateObjects(config *rest.Config) {
	normalClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	podTemplateClient := normalClient.CoreV1().PodTemplates("default")

	// Run against a 1.35 alpha cluster that enables workloadRef
	// FEATURE_GATES=GenericWorkload=true hack/local-up-cluster.sh
	setWorkloadRefPatch := []byte(`{"template":{"spec":{"workloadRef":{"name":"example.com","podGroup":"test"}}}}`)
	clearWorkloadRefPatch := []byte(`{"template":{"spec":{"workloadRef":null}}}`)

	// delete/create good
	{
		podTemplateClient.Delete(context.TODO(), "good", metav1.DeleteOptions{})
		deployment := &corev1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "good"},
			Template:   corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"key": "value"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}}},
		}
		if _, err := podTemplateClient.Create(context.TODO(), deployment, metav1.CreateOptions{}); err != nil {
			log.Fatal(err)
		}
	}
	// delete/create/patch bad
	{
		podTemplateClient.Delete(context.TODO(), "bad", metav1.DeleteOptions{})
		deployment := &corev1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "bad"},
			Template:   corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"key": "value"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}}},
		}
		if _, err := podTemplateClient.Create(context.TODO(), deployment, metav1.CreateOptions{}); err != nil {
			log.Fatal(err)
		}
		if _, err := podTemplateClient.Patch(context.TODO(), "bad", types.MergePatchType, setWorkloadRefPatch, metav1.PatchOptions{}); err != nil {
			log.Fatal(err)
		}
	}
	// create fluttering
	{
		podTemplateClient.Delete(context.TODO(), "fluttering", metav1.DeleteOptions{})
		deployment := &corev1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "fluttering"},
			Template:   corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"key": "value"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "test", Image: "test"}}}},
		}
		if _, err := podTemplateClient.Create(context.TODO(), deployment, metav1.CreateOptions{}); err != nil {
			log.Fatal(err)
		}
		if _, err := podTemplateClient.Patch(context.TODO(), "fluttering", types.MergePatchType, setWorkloadRefPatch, metav1.PatchOptions{}); err != nil {
			log.Fatal(err)
		}
		time.Sleep(time.Second)
		go func() {
			for {
				if _, err := podTemplateClient.Patch(context.TODO(), "fluttering", types.MergePatchType, setWorkloadRefPatch, metav1.PatchOptions{}); err != nil {
					log.Fatal(err)
				}
				time.Sleep(time.Second)
				if _, err := podTemplateClient.Patch(context.TODO(), "fluttering", types.MergePatchType, clearWorkloadRefPatch, metav1.PatchOptions{}); err != nil {
					log.Fatal(err)
				}
				time.Sleep(time.Second)
			}
		}()
	}
}
