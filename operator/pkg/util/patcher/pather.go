package patcher

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Patcher defines multiple variables that need to be patched.
type Patcher struct {
	labels      map[string]string
	annotations map[string]string
}

// NewPatcher returns a patcher.
func NewPatcher() *Patcher {
	return &Patcher{}
}

// WithLabels sets labels to the patcher.
func (p *Patcher) WithLabels(labels labels.Set) *Patcher {
	p.labels = labels
	return p
}

// WithAnnotations sets annotations to the patcher.
func (p *Patcher) WithAnnotations(annotations labels.Set) *Patcher {
	p.annotations = annotations
	return p
}

// ForDeployment patches the deployment manifest.
func (p *Patcher) ForDeployment(deployment *appsv1.Deployment) {
	deployment.Labels = labels.Merge(deployment.Labels, p.labels)
	deployment.Spec.Template.Labels = labels.Merge(deployment.Spec.Template.Labels, p.labels)

	deployment.Annotations = labels.Merge(deployment.Annotations, p.annotations)
	deployment.Spec.Template.Annotations = labels.Merge(deployment.Spec.Template.Annotations, p.annotations)
}

// ForStatefulSet patches the statefulset manifest.
func (p *Patcher) ForStatefulSet(sts *appsv1.StatefulSet) {
	sts.Labels = labels.Merge(sts.Labels, p.labels)
	sts.Spec.Template.Labels = labels.Merge(sts.Spec.Template.Labels, p.labels)

	sts.Annotations = labels.Merge(sts.Annotations, p.annotations)
	sts.Spec.Template.Annotations = labels.Merge(sts.Spec.Template.Annotations, p.annotations)
}
