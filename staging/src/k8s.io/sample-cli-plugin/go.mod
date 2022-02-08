// This is a generated file. Do not edit directly.

module k8s.io/sample-cli-plugin

go 1.16

require (
	github.com/liggitt/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	k8s.io/cli-runtime v0.0.0
	k8s.io/client-go v0.0.0
)

replace (
	github.com/spf13/viper => github.com/spf13/viper v1.10.0
	k8s.io/api => ../api
	k8s.io/apimachinery => ../apimachinery
	k8s.io/cli-runtime => ../cli-runtime
	k8s.io/client-go => ../client-go
	k8s.io/sample-cli-plugin => ../sample-cli-plugin
	sigs.k8s.io/kustomize/kyaml => github.com/liggitt/kustomize/kyaml v0.0.0-20220208204027-201e5cc8ba12
)
