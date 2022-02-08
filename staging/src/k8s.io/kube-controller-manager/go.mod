// This is a generated file. Do not edit directly.

module k8s.io/kube-controller-manager

go 1.16

require (
	k8s.io/apimachinery v0.0.0
	k8s.io/cloud-provider v0.0.0
	k8s.io/controller-manager v0.0.0
)

replace (
	github.com/spf13/viper => github.com/spf13/viper v1.10.0
	go.etcd.io/etcd/pkg/v3 => github.com/liggitt/etcd/pkg/v3 v3.0.0-20220208205624-4ad1b9fd4523
	go.etcd.io/etcd/server/v3 => github.com/liggitt/etcd/server/v3 v3.0.0-20220208205800-c5aadd525e88
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/apiserver => ../apiserver
	k8s.io/client-go => ../client-go
	k8s.io/cloud-provider => ../cloud-provider
	k8s.io/component-base => ../component-base
	k8s.io/component-helpers => ../component-helpers
	k8s.io/controller-manager => ../controller-manager
	k8s.io/kube-controller-manager => ../kube-controller-manager
)
