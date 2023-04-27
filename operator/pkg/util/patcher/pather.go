package patcher

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
)

// Patcher defines multiple variables that need to be patched.
type Patcher struct {
	labels      map[string]string
	annotations map[string]string
	volume      *operatorv1alpha1.VolumeData
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

// WithVolumeData sets VolumeData to the patcher.
func (p *Patcher) WithVolumeData(volume *operatorv1alpha1.VolumeData) *Patcher {
	p.volume = volume
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

	if p.volume != nil {
		patchVolumeForStatefulSet(sts, p.volume)
	}
}

func patchVolumeForStatefulSet(sts *appsv1.StatefulSet, volume *operatorv1alpha1.VolumeData) {
	if volume.EmptyDir != nil {
		volumes := sts.Spec.Template.Spec.Volumes
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
		sts.Spec.Template.Spec.Volumes = volumes
	}

	if volume.HostPath != nil {
		volumes := sts.Spec.Template.Spec.Volumes
		volumes = append(volumes, corev1.Volume{
			Name: constants.EtcdDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: volume.HostPath.Path,
					Type: volume.HostPath.Type,
				},
			},
		})
		sts.Spec.Template.Spec.Volumes = volumes
	}

	if volume.VolumeClaim != nil {
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.EtcdDataVolumeName,
				},
				Spec: volume.VolumeClaim.Spec,
			},
		}
	}
}
