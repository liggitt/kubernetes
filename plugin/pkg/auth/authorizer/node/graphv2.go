/*
Copyright 2026 The Kubernetes Authors.

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

package node

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"unique"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/component-helpers/storage/ephemeral"
	"k8s.io/dynamic-resource-allocation/resourceclaim"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

// GraphV2 holds ref counts and a way to look up a vertex for a particular API type/namespace/name.
type GraphV2 struct {
	// holds all the refcounts for everything used by authorization checks
	nodes sync.Map // map[string]*references

	// pvc index into the record for the PVC
	pvcs map[refKey]*references
}

func NewGraphV2() *GraphV2 {
	return &GraphV2{}
}

type typeNamespaceName struct {
	objectType      vertexType
	objectNamespace unique.Handle[string]
	objectName      unique.Handle[string]
}
type refKey = unique.Handle[typeNamespaceName]
type references struct {
	lock       sync.RWMutex
	references map[refKey]*atomic.Int32
}

// UpdatePod should only be called once spec.NodeName is populated.
func (g *GraphV2) UpdatePod(oldPod, pod *corev1.Pod) {
	start := time.Now()
	defer func() {
		graphActionsDuration.WithLabelValues("UpdatePod").Observe(time.Since(start).Seconds())
	}()
	oldNode, oldReferences := referencesFromPod(oldPod)
	newNode, newReferences := referencesFromPod(pod)
	g.applyDelta(oldNode, newNode, oldReferences, newReferences)
}

func referencesFromPod(pod *corev1.Pod) (string, map[refKey]int) {
	if pod == nil || pod.Spec.NodeName == "" {
		return "", nil
	}
	retval := map[refKey]int{
		ref(podVertexType, pod.Namespace, pod.Name): 1,
	}

	// Short-circuit adding edges to other resources for mirror pods.
	// A node must never be able to create a pod that grants them permissions on other API objects.
	// The NodeRestriction admission plugin prevents creation of such pods, but short-circuiting here gives us defense in depth.
	if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
		return pod.Spec.NodeName, retval
	}

	// The pod unconditionally gets access to the pod's service account.  In the
	// future, this access could be restricted based on whether or not the pod
	// actually mounts a service account token, or has a podcertificate volume.
	if len(pod.Spec.ServiceAccountName) > 0 {
		retval[ref(serviceAccountVertexType, pod.Namespace, pod.Spec.ServiceAccountName)] = 1
	}

	// Secrets
	podutil.VisitPodSecretNames(pod, func(secret string) bool {
		retval[ref(secretVertexType, pod.Namespace, secret)] = 1
		return true
	})

	// ConfigMaps
	podutil.VisitPodConfigmapNames(pod, func(configmap string) bool {
		retval[ref(configMapVertexType, pod.Namespace, configmap)] = 1
		return true
	})

	// PVCs
	for _, v := range pod.Spec.Volumes {
		claimName := ""
		if v.PersistentVolumeClaim != nil {
			claimName = v.PersistentVolumeClaim.ClaimName
		} else if v.Ephemeral != nil {
			claimName = ephemeral.VolumeClaimName(pod, &v)
		}
		if claimName != "" {
			retval[ref(pvcVertexType, pod.Namespace, claimName)] = 1
		}
	}

	// ResourceClaims
	// Grant kubelet access to all ResourceClaims referenced by or created for
	// the pod (including extended resource claims). Claims that still need to
	// be created or intentionally were not created because they are not needed
	// are skipped by PodClaims.
	for claimName := range resourceclaim.PodClaims(pod) {
		retval[ref(resourceClaimVertexType, pod.Namespace, claimName)] = 1
	}

	return pod.Spec.NodeName, retval
}

func (g *GraphV2) UpdatePV(oldPV, pv *corev1.PersistentVolume) {
	start := time.Now()
	defer func() {
		graphActionsDuration.WithLabelValues("UpdatePV").Observe(time.Since(start).Seconds())
	}()
	// update the pv entry references
	// update the pv entry in pvc entry
	// update pv+pvsecret entries in referenced nodes for old/new pvc
}

func (g *GraphV2) UpdateVolumeAttachment(oldAttachment, attachment *storagev1.VolumeAttachment) {
	start := time.Now()
	defer func() {
		graphActionsDuration.WithLabelValues("UpdateVolumeAttachment").Observe(time.Since(start).Seconds())
	}()
}

func (g *GraphV2) UpdateResourceSlice(oldSlice, slice *resourcev1.ResourceSlice) {
	start := time.Now()
	defer func() {
		graphActionsDuration.WithLabelValues("UpdateVolumeAttachment").Observe(time.Since(start).Seconds())
	}()
}

func (g *GraphV2) UpdatePodCertificateRequest(oldPCR, pcr *certificatesv1.PodCertificateRequest) {
	start := time.Now()
	defer func() {
		graphActionsDuration.WithLabelValues("UpdateVolumeAttachment").Observe(time.Since(start).Seconds())
	}()
}

func (g *GraphV2) applyDelta(oldNode, newNode string, oldReferences, newReferences map[refKey]int) {
	// Same node, merge and discard unchanged entries
	if newNode == oldNode && len(newReferences) > 0 {
		for k, v := range oldReferences {
			if newReferences[k] == v {
				delete(newReferences, k)
			} else {
				newReferences[k] -= v
			}
		}
		oldReferences = nil
	}

	if len(newReferences) > 0 {
		g.ensureNodeRecord(newNode).addReferences(newReferences)
	}
	if len(oldReferences) > 0 {
		g.ensureNodeRecord(oldNode).subtractReferences(oldReferences)
	}
}

func (r *references) addReferences(references map[refKey]int) {
	for k, v := range references {
		r.ensureCounter(k).Add(int32(v))
		// TODO: prune empty counter
	}
	// TODO: prune empty references
}
func (r *references) subtractReferences(references map[refKey]int) {
	for k, v := range references {
		r.ensureCounter(k).Add(-int32(v))
		// TODO: prune empty counter
	}
	// TODO: prune empty references
}
func (r *references) hasRef(key refKey) bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	value, ok := r.references[key]
	return ok && value.Load() > 0
}
func (r *references) ensureCounter(key refKey) *atomic.Int32 {
	r.lock.RLock()
	value, ok := r.references[key]
	r.lock.RUnlock()
	if ok {
		return value
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	value, ok = r.references[key]
	if ok {
		return value
	}

	value = &atomic.Int32{}
	r.references[key] = value
	return value
}

func (g *GraphV2) getNodeRecord(name string) (*references, bool) {
	value, ok := g.nodes.Load(name)
	if !ok {
		return nil, false
	}
	return value.(*references), true
}
func (g *GraphV2) ensureNodeRecord(name string) *references {
	value, ok := g.nodes.Load(name)
	if !ok {
		value, _ = g.nodes.LoadOrStore(name, &references{})
	}
	return value.(*references)
}

func ref(objectType vertexType, objectNamespace, objectName string) refKey {
	return refKey(unique.Make(typeNamespaceName{objectType, unique.Make(objectNamespace), unique.Make(objectName)}))
}

// relatedToObject returns true if the specified node is related to the specified type/namespace/name
func (g *GraphV2) relatedToObject(nodeName string, objectType vertexType, objectNamespace, objectName string) (bool, error) {
	nodeRecord, exists := g.getNodeRecord(nodeName)
	if !exists {
		return false, fmt.Errorf("unknown node '%s' cannot get %s %s/%s", nodeName, vertexTypes[objectType], objectNamespace, objectName)
	}
	if !nodeRecord.hasRef(ref(objectType, objectNamespace, objectName)) {
		return false, fmt.Errorf("node '%s' cannot get %s %s/%s, no relationship to this object was found in the node authorizer graph", nodeName, vertexTypes[objectType], objectNamespace, objectName)
	}
	return true, nil
}
