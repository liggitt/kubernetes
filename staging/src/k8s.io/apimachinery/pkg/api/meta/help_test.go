/*
Copyright 2015 The Kubernetes Authors.

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

package meta

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	fakeObjectItemsNum = 1000
)

type FooSpec struct {
	Flied int
}

func (f Foo) DeepCopyObject() runtime.Object { return nil }

type FooList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Foo
}

func (s *FooList) DeepCopyObject() runtime.Object { return nil }

// The difference between Sample and Foo is that the pointer of Sample
// is the implementer of runtime.Object, while the Foo struct itself is
// the implementer of runtime.Object. This difference affects the
// behavior of ExtractList.
type Sample struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec SampleSpec
}

type Foo struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec FooSpec
}

type SampleSpec struct {
	Flied int
}

func (s *Sample) DeepCopyObject() runtime.Object { return nil }

type SampleList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Sample
}

func (s *SampleList) DeepCopyObject() runtime.Object { return nil }

type RawExtensionList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []runtime.RawExtension
}

func (l RawExtensionList) DeepCopyObject() runtime.Object { return nil }

func getSampleList(numItems int) *SampleList {
	out := &SampleList{
		Items: make([]Sample, numItems),
	}

	for i := range out.Items {
		out.Items[i] = Sample{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "foo.org/v1",
				Kind:       "Sample",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("sample-%d", i),
				Namespace: "default",
				Labels: map[string]string{
					"label-key-1": "label-value-1",
				},
				Annotations: map[string]string{
					"annotations-key-1": "annotations-value-1",
				},
			},
			Spec: SampleSpec{
				Flied: i,
			},
		}
	}
	return out
}

func getRawExtensionList(numItems int) *RawExtensionList {
	out := &RawExtensionList{
		Items: make([]runtime.RawExtension, numItems),
	}

	for i := range out.Items {
		out.Items[i] = runtime.RawExtension{
			Object: &Foo{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "sample.org/v1",
					Kind:       "Foo",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("foo-%d", i),
					Namespace: "default",
					Labels: map[string]string{
						"label-key-1": "label-value-1",
					},
					Annotations: map[string]string{
						"annotations-key-1": "annotations-value-1",
					},
				},
				Spec: FooSpec{
					Flied: i,
				},
			},
		}
	}
	return out
}

func getFooList(numItems int) *FooList {
	out := &FooList{
		Items: make([]Foo, numItems),
	}

	for i := range out.Items {
		out.Items[i] = Foo{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "sample.org/v1",
				Kind:       "Foo",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("foo-%d", i),
				Namespace: "default",
				Labels: map[string]string{
					"label-key-1": "label-value-1",
				},
				Annotations: map[string]string{
					"annotations-key-1": "annotations-value-1",
				},
			},
			Spec: FooSpec{
				Flied: i,
			},
		}
	}
	return out
}

func BenchmarkExtractSampleList(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExtractList(getSampleList(fakeObjectItemsNum))
		if err != nil {
			b.Fatalf("extract pod list: %v", err)
		}
	}
	b.StopTimer()
}

func BenchmarkExtractFooList(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExtractList(getFooList(fakeObjectItemsNum))
		if err != nil {
			b.Fatalf("extract pod list: %v", err)
		}
	}
	b.StopTimer()
}

func BenchmarkExtractRawExtensionList(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExtractList(getRawExtensionList(fakeObjectItemsNum))
		if err != nil {
			b.Fatalf("extract pod list: %v", err)
		}
	}
	b.StopTimer()
}
