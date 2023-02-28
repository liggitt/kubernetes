/*
Copyright 2014 The Kubernetes Authors.

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

package config

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	podutil "k8s.io/kubernetes/pkg/api/pod"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/helper"

	// TODO: remove this import if
	// api.Registry.GroupOrDie(v1.GroupName).GroupVersion.String() is changed
	// to "v1"?
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	// Ensure that core apis are installed
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	k8s_api_v1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/apis/core/validation"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/util/hash"

	"k8s.io/klog/v2"
)

const (
	maxConfigLength = 10 * 1 << 20 // 10MB
)

// Generate a pod name that is unique among nodes by appending the nodeName.
func generatePodName(name string, nodeName types.NodeName) string {
	return fmt.Sprintf("%s-%s", name, strings.ToLower(string(nodeName)))
}

func applyDefaults(pod *api.Pod, source string, isFile bool, nodeName types.NodeName) error {
	if len(pod.UID) == 0 {
		hasher := md5.New()
		hash.DeepHashObject(hasher, pod)
		// DeepHashObject resets the hash, so we should write the pod source
		// information AFTER it.
		if isFile {
			fmt.Fprintf(hasher, "host:%s", nodeName)
			fmt.Fprintf(hasher, "file:%s", source)
		} else {
			fmt.Fprintf(hasher, "url:%s", source)
		}
		pod.UID = types.UID(hex.EncodeToString(hasher.Sum(nil)[0:]))
		klog.V(5).InfoS("Generated UID", "pod", klog.KObj(pod), "podUID", pod.UID, "source", source)
	}

	pod.Name = generatePodName(pod.Name, nodeName)
	klog.V(5).InfoS("Generated pod name", "pod", klog.KObj(pod), "podUID", pod.UID, "source", source)

	if pod.Namespace == "" {
		pod.Namespace = metav1.NamespaceDefault
	}
	klog.V(5).InfoS("Set namespace for pod", "pod", klog.KObj(pod), "source", source)

	// Set the Host field to indicate this pod is scheduled on the current node.
	pod.Spec.NodeName = string(nodeName)

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	// The generated UID is the hash of the file.
	pod.Annotations[kubetypes.ConfigHashAnnotationKey] = string(pod.UID)

	if isFile {
		// Applying the default Taint tolerations to static pods,
		// so they are not evicted when there are node problems.
		helper.AddOrUpdateTolerationInPod(pod, &api.Toleration{
			Operator: "Exists",
			Effect:   api.TaintEffectNoExecute,
		})
	}

	// Set the default status to pending.
	pod.Status.Phase = api.PodPending
	return nil
}

type defaultFunc func(pod *api.Pod) error

// tryDecodeSinglePod takes data and tries to extract valid Pod config information from it.
func tryDecodeSinglePod(data []byte, defaultFn defaultFunc) (parsed bool, pod *v1.Pod, err error) {
	// JSON is valid YAML, so this should work for everything.
	json, err := utilyaml.ToJSON(data)
	if err != nil {
		return false, nil, err
	}
	obj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), json)
	if err != nil {
		return false, pod, err
	}

	newPod, ok := obj.(*api.Pod)
	// Check whether the object could be converted to single pod.
	if !ok {
		return false, pod, fmt.Errorf("invalid pod: %#v", obj)
	}

	// Apply default values and validate the pod.
	if err = defaultFn(newPod); err != nil {
		return true, pod, err
	}
	if errs := validation.ValidatePodCreate(newPod, validation.PodValidationOptions{}); len(errs) > 0 {
		return true, pod, fmt.Errorf("invalid pod: %v", errs)
	}

	v1Pod := &v1.Pod{}
	if err := k8s_api_v1.Convert_core_Pod_To_v1_Pod(newPod, v1Pod, nil); err != nil {
		klog.ErrorS(err, "Pod failed to convert to v1", "pod", klog.KObj(newPod))
		return true, nil, err
	}

	// Ensure no API objects are referenced from the pod
	if apiReferences := getAPIReferences(newPod); len(apiReferences) > 0 {
		return true, nil, fmt.Errorf("pod %q has forbidden references to API objects: %s", pod.Name, strings.Join(apiReferences, ", "))
	}

	return true, v1Pod, nil
}

func getAPIReferences(pod *api.Pod) []string {
	apiReferences := sets.NewString()
	if len(pod.Spec.ServiceAccountName) > 0 {
		apiReferences.Insert(fmt.Sprintf("ServiceAccount %q", pod.Spec.ServiceAccountName))
	}
	podutil.VisitPodSecretNames(pod, func(name string) (shouldContinue bool) {
		apiReferences.Insert(fmt.Sprintf("Secret %q", name))
		return true
	}, podutil.AllContainers)
	podutil.VisitPodConfigmapNames(pod, func(name string) (shouldContinue bool) {
		apiReferences.Insert(fmt.Sprintf("ConfigMap %q", name))
		return true
	}, podutil.AllContainers)
	for _, v := range pod.Spec.Volumes {
		switch {
		case v.AWSElasticBlockStore != nil, v.AzureDisk != nil, v.AzureFile != nil, v.CephFS != nil, v.Cinder != nil,
			v.DownwardAPI != nil, v.EmptyDir != nil, v.FC != nil, v.FlexVolume != nil, v.Flocker != nil, v.GCEPersistentDisk != nil,
			v.GitRepo != nil, v.Glusterfs != nil, v.HostPath != nil, v.ISCSI != nil, v.NFS != nil, v.PhotonPersistentDisk != nil,
			v.PortworxVolume != nil, v.Quobyte != nil, v.RBD != nil, v.ScaleIO != nil, v.StorageOS != nil, v.VsphereVolume != nil:
			// Allow volume types that don't require the Kubernetes API
		case v.Projected != nil:
			for _, s := range v.Projected.Sources {
				// Reject projected volume sources that require the Kubernetes API
				switch {
				case s.ConfigMap != nil:
					apiReferences.Insert(fmt.Sprintf("configMap projected volume %q", v.Name))
				case s.Secret != nil:
					apiReferences.Insert(fmt.Sprintf("secret projected volume %q", v.Name))
				case s.ServiceAccountToken != nil:
					apiReferences.Insert(fmt.Sprintf("serviceAccountToken projected volume %q", v.Name))
				case s.DownwardAPI != nil:
					// Allow projected volume sources that don't require the Kubernetes API
				default:
					// Reject unknown volume types
					apiReferences.Insert(fmt.Sprintf("unknown source for projected volume %q", v.Name))
				}
			}
		case v.ConfigMap != nil:
			apiReferences.Insert(fmt.Sprintf("ConfigMap volume %q", v.Name))
		case v.CSI != nil:
			apiReferences.Insert(fmt.Sprintf("CSI volume %q", v.Name))
		case v.Ephemeral != nil:
			apiReferences.Insert(fmt.Sprintf("Ephemeral volume %q", v.Name))
		case v.PersistentVolumeClaim != nil:
			apiReferences.Insert(fmt.Sprintf("PersistentVolumeClaim volume %q", v.Name))
		case v.Secret != nil:
			apiReferences.Insert(fmt.Sprintf("Secret volume %q", v.Name))
		default:
			// Reject unknown volume types
			apiReferences.Insert(fmt.Sprintf("unknown type for volume %q", v.Name))
		}
	}
	return apiReferences.List()
}

func tryDecodePodList(data []byte, defaultFn defaultFunc) (parsed bool, pods v1.PodList, err error) {
	obj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
	if err != nil {
		return false, pods, err
	}

	newPods, ok := obj.(*api.PodList)
	// Check whether the object could be converted to list of pods.
	if !ok {
		err = fmt.Errorf("invalid pods list: %#v", obj)
		return false, pods, err
	}

	// Apply default values and validate pods.
	for i := range newPods.Items {
		newPod := &newPods.Items[i]
		if err = defaultFn(newPod); err != nil {
			return true, pods, err
		}
		if errs := validation.ValidatePodCreate(newPod, validation.PodValidationOptions{}); len(errs) > 0 {
			err = fmt.Errorf("invalid pod: %v", errs)
			return true, pods, err
		}
	}
	v1Pods := &v1.PodList{}
	if err := k8s_api_v1.Convert_core_PodList_To_v1_PodList(newPods, v1Pods, nil); err != nil {
		return true, pods, err
	}
	return true, *v1Pods, err
}
