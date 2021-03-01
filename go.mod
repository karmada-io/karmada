module github.com/karmada-io/karmada

go 1.14

require (
	github.com/evanphx/json-patch/v5 v5.1.0
	github.com/go-logr/logr v0.3.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/onsi/ginkgo v1.12.1
	github.com/onsi/gomega v1.10.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.19.3
	k8s.io/apiextensions-apiserver v0.19.3
	k8s.io/apimachinery v0.19.3
	k8s.io/apiserver v0.19.3
	k8s.io/client-go v0.19.3
	k8s.io/code-generator v0.19.3
	k8s.io/component-base v0.19.3
	k8s.io/klog/v2 v2.2.0
	k8s.io/utils v0.0.0-20200729134348-d5654de09c73
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/kind v0.10.0
)
