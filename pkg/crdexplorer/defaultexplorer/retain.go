package defaultexplorer

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RetentionExplorer is the function that retains values from "Cluster" object.
type RetentionExplorer func(desired *unstructured.Unstructured, cluster *unstructured.Unstructured) (retained *unstructured.Unstructured, err error)

func getAllDefaultRetentionExplorer() map[schema.GroupVersionKind]RetentionExplorer {
	explorers := make(map[schema.GroupVersionKind]RetentionExplorer)

	// TODO(RainbowMango): Migrate retention functions from
	// https://github.com/karmada-io/karmada/blob/master/pkg/util/objectwatcher/retain.go
	return explorers
}
