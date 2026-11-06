/*
Copyright 2017 The Kubernetes Authors.

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
	"context"

	certsv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	storagev1 "k8s.io/api/storage/v1"
	certsv1informers "k8s.io/client-go/informers/certificates/v1"
	corev1informers "k8s.io/client-go/informers/core/v1"
	resourceinformers "k8s.io/client-go/informers/resource/v1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/dynamic-resource-allocation/resourceclaim"
	"k8s.io/klog/v2"
)

type graphV2Populator struct {
	graph *GraphV2
}

func AddGraphV2EventHandlers(
	ctx context.Context,
	graph *GraphV2,
	nodes corev1informers.NodeInformer,
	pods corev1informers.PodInformer,
	pvs corev1informers.PersistentVolumeInformer,
	attachments storageinformers.VolumeAttachmentInformer,
	slices resourceinformers.ResourceSliceInformer,
	pcrs certsv1informers.PodCertificateRequestInformer,
) {
	g := &graphV2Populator{graph: graph}

	podHandler, _ := pods.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    g.addPod,
		UpdateFunc: g.updatePod,
		DeleteFunc: g.deletePod,
	})

	pvsHandler, _ := pvs.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    g.addPV,
		UpdateFunc: g.updatePV,
		DeleteFunc: g.deletePV,
	})

	attachHandler, _ := attachments.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    g.addVolumeAttachment,
		UpdateFunc: g.updateVolumeAttachment,
		DeleteFunc: g.deleteVolumeAttachment,
	})

	sliceHandler, _ := slices.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    g.addResourceSlice,
		UpdateFunc: g.updateResourceSlice,
		DeleteFunc: g.deleteResourceSlice,
	})

	synced := []cache.InformerSynced{
		podHandler.HasSynced, pvsHandler.HasSynced, attachHandler.HasSynced, sliceHandler.HasSynced,
	}

	if pcrs != nil {
		pcrHandler, _ := pcrs.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    g.addPCR,
			UpdateFunc: g.updatePCR,
			DeleteFunc: g.deletePCR,
		})
		synced = append(synced, pcrHandler.HasSynced)
	}

	go cache.WaitForNamedCacheSync("node_authorizer", ctx.Done(), synced...)
}

func (g *graphV2Populator) addPod(obj interface{}) {
	g.updatePod(nil, obj)
}

func (g *graphV2Populator) updatePod(oldObj, obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	oldPod, _ := oldObj.(*corev1.Pod)

	if len(pod.Spec.NodeName) == 0 && (oldPod == nil || len(oldPod.Spec.NodeName) == 0) {
		// No node assigned
		klog.V(5).Infof("updatePod %s/%s, no node", pod.Namespace, pod.Name)
		return
	}

	if oldPod != nil {
		if (pod.Spec.NodeName == oldPod.Spec.NodeName) && (pod.UID == oldPod.UID) &&
			!podSpecChanged(oldPod, pod) &&
			resourceclaim.PodStatusEqual(oldPod.Status.ResourceClaimStatuses, pod.Status.ResourceClaimStatuses) &&
			resourceclaim.PodExtendedStatusEqual(oldPod.Status.ExtendedResourceClaimStatus, pod.Status.ExtendedResourceClaimStatus) {
			// Node and uid are unchanged, and all object references in the pod spec and status are unmodified.
			klog.V(5).Infof("updatePod %s/%s, node unchanged", pod.Namespace, pod.Name)
			return
		}
	}

	g.graph.UpdatePod(oldPod, pod)
}

func (g *graphV2Populator) deletePod(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	if len(pod.Spec.NodeName) == 0 {
		klog.V(5).Infof("deletePod %s/%s, no node", pod.Namespace, pod.Name)
		return
	}

	g.graph.UpdatePod(nil, pod)
}

func (g *graphV2Populator) addPV(obj interface{}) {
	g.updatePV(nil, obj)
}

func (g *graphV2Populator) updatePV(oldObj, obj interface{}) {
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	oldPV, _ := obj.(*corev1.PersistentVolume)
	g.graph.UpdatePV(oldPV, pv)
}

func (g *graphV2Populator) deletePV(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	pv, ok := obj.(*corev1.PersistentVolume)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	g.graph.UpdatePV(pv, nil)
}

func (g *graphV2Populator) addVolumeAttachment(obj interface{}) {
	g.updateVolumeAttachment(nil, obj)
}

func (g *graphV2Populator) updateVolumeAttachment(oldObj, obj interface{}) {
	attachment, ok := obj.(*storagev1.VolumeAttachment)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	oldAttachment, _ := oldObj.(*storagev1.VolumeAttachment)
	if oldAttachment != nil {
		// skip add if node name is identical
		if oldAttachment.Spec.NodeName == attachment.Spec.NodeName {
			return
		}
	}
	g.graph.UpdateVolumeAttachment(oldAttachment, attachment)
}

func (g *graphV2Populator) deleteVolumeAttachment(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	attachment, ok := obj.(*storagev1.VolumeAttachment)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	g.graph.UpdateVolumeAttachment(attachment, nil)
}

func (g *graphV2Populator) addResourceSlice(obj interface{}) {
	g.updateResourceSlice(nil, obj)
}

func (g *graphV2Populator) updateResourceSlice(oldObj, obj interface{}) {
	slice, ok := obj.(*resourceapi.ResourceSlice)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	oldSlice, _ := oldObj.(*resourceapi.ResourceSlice)
	g.graph.UpdateResourceSlice(oldSlice, slice)
}

func (g *graphV2Populator) deleteResourceSlice(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	slice, ok := obj.(*resourceapi.ResourceSlice)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	g.graph.UpdateResourceSlice(slice, nil)
}

func (g *graphV2Populator) addPCR(obj any) {
	g.updatePCR(nil, obj)
}
func (g *graphV2Populator) updatePCR(oldObj, obj any) {
	pcr, ok := obj.(*certsv1.PodCertificateRequest)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	oldPCR, _ := oldObj.(*certsv1.PodCertificateRequest)
	g.graph.UpdatePodCertificateRequest(oldPCR, pcr)
}

func (g *graphV2Populator) deletePCR(obj any) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	pcr, ok := obj.(*certsv1.PodCertificateRequest)
	if !ok {
		klog.Infof("unexpected type %T", obj)
		return
	}
	g.graph.UpdatePodCertificateRequest(pcr, nil)
}
