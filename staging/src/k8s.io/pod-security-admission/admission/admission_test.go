/*
Copyright 2021 The Kubernetes Authors.

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

package admission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefaultExtractPodSpec(t *testing.T) {
	metadata := metav1.ObjectMeta{
		Name: "foo-pod",
	}
	spec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "foo-container",
		}},
	}
	objects := []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metadata,
			Spec:       spec,
		},
		&corev1.PodTemplate{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-template"},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metadata,
				Spec:       spec,
			},
		},
		&corev1.ReplicationController{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-rc"},
			Spec: corev1.ReplicationControllerSpec{
				Template: &corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-rs"},
			Spec: appsv1.ReplicaSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-deployment"},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-ss"},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-ds"},
			Spec: appsv1.DaemonSetSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-job"},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metadata,
					Spec:       spec,
				},
			},
		},
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-cronjob"},
			Spec: batchv1.CronJobSpec{
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metadata,
							Spec:       spec,
						},
					},
				},
			},
		},
	}
	extractor := &DefaultPodSpecExtractor{}
	for _, obj := range objects {
		name := obj.(metav1.Object).GetName()
		actualMetadata, actualSpec, err := extractor.ExtractPodSpec(obj)
		assert.NoError(t, err, name)
		assert.Equal(t, &metadata, actualMetadata, "%s: Metadata mismatch", name)
		assert.Equal(t, &spec, actualSpec, "%s: PodSpec mismatch", name)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo-svc",
		},
	}
	_, _, err := extractor.ExtractPodSpec(service)
	assert.Error(t, err, "service should not have an extractable pod spec")
}

func TestDefaultHasPodSpec(t *testing.T) {
	podLikeResources := []schema.GroupResource{
		corev1.Resource("pods"),
		corev1.Resource("replicationcontrollers"),
		corev1.Resource("podtemplates"),
		appsv1.Resource("replicasets"),
		appsv1.Resource("deployments"),
		appsv1.Resource("statefulsets"),
		appsv1.Resource("daemonsets"),
		batchv1.Resource("jobs"),
		batchv1.Resource("cronjobs"),
	}
	extractor := &DefaultPodSpecExtractor{}
	for _, gr := range podLikeResources {
		assert.True(t, extractor.HasPodSpec(gr), gr.String())
	}

	nonPodResources := []schema.GroupResource{
		corev1.Resource("services"),
		admissionv1.Resource("admissionreviews"),
		appsv1.Resource("foobars"),
	}
	for _, gr := range nonPodResources {
		assert.False(t, extractor.HasPodSpec(gr), gr.String())
	}
}
