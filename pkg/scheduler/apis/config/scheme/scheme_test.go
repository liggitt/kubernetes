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
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
)

func TestCodecsPluginConfig(t *testing.T) {
	testCases := []struct {
		name         string
		data         []byte
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
	}
	decoder := Codecs.UniversalDecoder()
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			obj, gvk, err := decoder.Decode(tt.data, nil, nil)
			if err != nil {
				t.Fatal(err)
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
