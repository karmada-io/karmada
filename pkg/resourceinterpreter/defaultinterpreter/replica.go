package defaultinterpreter

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

// replicaInterpreter is the function that used to parse replica and requirements from object.
type replicaInterpreter func(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error)

func getAllDefaultReplicaInterpreter() map[schema.GroupVersionKind]replicaInterpreter {
	s := make(map[schema.GroupVersionKind]replicaInterpreter)
	s[appsv1.SchemeGroupVersion.WithKind(util.DeploymentKind)] = deployReplica
	s[appsv1.SchemeGroupVersion.WithKind(util.StatefulSetKind)] = statefulSetReplica
	s[batchv1.SchemeGroupVersion.WithKind(util.JobKind)] = jobReplica
	return s
}

func deployReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	deploy := &appsv1.Deployment{}
	if err := helper.ConvertToTypedObject(object, deploy); err != nil {
		klog.Errorf("Failed to convert object(%s), err", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	if deploy.Spec.Replicas != nil {
		replica = *deploy.Spec.Replicas
	}
	requirement := helper.GenerateReplicaRequirements(&deploy.Spec.Template)

	return replica, requirement, nil
}

func statefulSetReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	sts := &appsv1.StatefulSet{}
	if err := helper.ConvertToTypedObject(object, sts); err != nil {
		klog.Errorf("Failed to convert object(%s), err", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	if sts.Spec.Replicas != nil {
		replica = *sts.Spec.Replicas
	}
	requirement := helper.GenerateReplicaRequirements(&sts.Spec.Template)

	return replica, requirement, nil
}

func jobReplica(object *unstructured.Unstructured) (int32, *workv1alpha2.ReplicaRequirements, error) {
	job := &batchv1.Job{}
	err := helper.ConvertToTypedObject(object, job)
	if err != nil {
		klog.Errorf("Failed to convert object(%s), err", object.GroupVersionKind().String(), err)
		return 0, nil, err
	}

	var replica int32
	// parallelism might never be nil as the kube-apiserver will set it to 1 by default if not specified.
	if job.Spec.Parallelism != nil {
		replica = *job.Spec.Parallelism
	}
	requirement := helper.GenerateReplicaRequirements(&job.Spec.Template)

	return replica, requirement, nil
}
