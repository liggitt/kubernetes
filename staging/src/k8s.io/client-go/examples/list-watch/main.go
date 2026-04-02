/*
Copyright The Kubernetes Authors.

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

// Note: the example only works with the code within the same release/branch.
package main

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	w, err := clientset.CoreV1().Pods("").Watch(context.TODO(), metav1.ListOptions{
		Watch:                true,
		AllowWatchBookmarks:  true,
		SendInitialEvents:    new(true),
		ResourceVersion:      "",
		ResourceVersionMatch: metav1.ResourceVersionMatchNotOlderThan,
	})
	if err != nil {
		panic(err.Error())
	}

	completed := false
	resourceVersion := ""
	podCount := 0

loop:
	for event := range w.ResultChan() {
		switch event.Type {
		case watch.Added:
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				fmt.Printf("unexpected type received: %T\n", event.Object)
				break loop
			}
			fmt.Printf("pod %s/%s\n", pod.Name, pod.Namespace)
			podCount++

		case watch.Bookmark:
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				fmt.Printf("unexpected type received: %T\n", event.Object)
				break loop
			}
			if pod.Annotations[metav1.InitialEventsAnnotationKey] == "true" {
				completed = true
				resourceVersion = pod.ResourceVersion
				break loop
			}

		case watch.Error:
			fmt.Printf("error received: %v\n", event.Object)
			break loop
		}
	}
	// stop in case we broke the loop before the channel closed
	w.Stop()

	if !completed {
		fmt.Println("watch ended before initial events were received")
	} else {
		fmt.Printf("streamed %d pods and at resourceVersion=%s\n", podCount, resourceVersion)
	}
}
