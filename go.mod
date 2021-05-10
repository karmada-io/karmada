module github.com/karmada-io/karmada

go 1.14

require (
	github.com/distribution/distribution/v3 v3.0.0-20210507173845-9329f6a62b67
	github.com/evanphx/json-patch/v5 v5.1.0
	github.com/google/uuid v1.1.2
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/apiserver v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/code-generator v0.20.6
	k8s.io/component-base v0.20.6
	k8s.io/klog/v2 v2.4.0
	k8s.io/utils v0.0.0-20210111153108-fddb29f9d009
	sigs.k8s.io/controller-runtime v0.8.3
	sigs.k8s.io/kind v0.10.0
)

// controller-runtime@v0.8.3 uses gnostic@v0.5.1 which not compatible with kubernetes@v1.20.2.
// kubernetes@v1.20.2 using gnostic@v0.4.1.
replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
