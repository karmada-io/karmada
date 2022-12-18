package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

// Scheme holds the aggregated Kubernetes's schemes and extended schemes.
var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(scheme.AddToScheme(Scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(Scheme))
}
