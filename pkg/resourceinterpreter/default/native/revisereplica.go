package native

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

type reviseReplicaInterpreter func(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error)

func getAllDefaultReviseReplicaInterpreter() map[schema.GroupVersionKind]reviseReplicaInterpreter {
	s := make(map[schema.GroupVersionKind]reviseReplicaInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = reviseDeploymentReplica
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = reviseStatefulSetReplica
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = reviseJobReplica
	return s
}

func reviseDeploymentReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	if err := helper.ApplyReplica(object, replica, util.ReplicasField); err != nil {
		return nil, err
	}
	return object, nil
}

func reviseStatefulSetReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	if err := helper.ApplyReplica(object, replica, util.ReplicasField); err != nil {
		return nil, err
	}
	return object, nil
}

func reviseJobReplica(object *unstructured.Unstructured, replica int64) (*unstructured.Unstructured, error) {
	if err := helper.ApplyReplica(object, replica, util.ParallelismField); err != nil {
		return nil, err
	}
	return object, nil
}
