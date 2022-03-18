package applyer

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/karmada-io/karmada/pkg/util/informermanager/keys"
)

// Applyer is the interface that wraps Apply method.
type Applyer interface {
	Apply(object *unstructured.Unstructured, objectKey keys.ClusterWideKey, policy *unstructured.Unstructured) error
}
