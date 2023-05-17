package lifted

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/utils/pointer"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#LL151C1-L186C2

// generateDeployment creates a deployment, with the input image as its template
func generateDeployment(image string) appsv1.Deployment {
	podLabels := map[string]string{"name": image}
	terminationSec := int64(30)
	enableServiceLinks := corev1.DefaultEnableServiceLinks
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        image,
			Annotations: make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(1),
			Selector: &metav1.LabelSelector{MatchLabels: podLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:                   image,
							Image:                  image,
							ImagePullPolicy:        corev1.PullAlways,
							TerminationMessagePath: corev1.TerminationMessagePathDefault,
						},
					},
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: &terminationSec,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SecurityContext:               &corev1.PodSecurityContext{},
					EnableServiceLinks:            &enableServiceLinks,
				},
			},
		},
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#LL129C1-L145C2
// +lifted:changed

// generateRS creates a replica set, with the input deployment's template as its template
func generateRS(deployment appsv1.Deployment) appsv1.ReplicaSet {
	template := deployment.Spec.Template.DeepCopy()
	return appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			UID:             "test",
			Name:            names.SimpleNameGenerator.GenerateName("replicaset"),
			Labels:          template.Labels,
			OwnerReferences: []metav1.OwnerReference{*newDControllerRef(&deployment)},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: new(int32),
			Template: *template,
			Selector: &metav1.LabelSelector{MatchLabels: template.Labels},
		},
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#LL118C1-L127C2

func newDControllerRef(d *appsv1.Deployment) *metav1.OwnerReference {
	isController := true
	return &metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       d.GetName(),
		UID:        d.GetUID(),
		Controller: &isController,
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#L326

func generatePodTemplateSpec(name, nodeName string, annotations, labels map[string]string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
}

func TestReplicaSetsByCreationTimestamp_Len(t *testing.T) {
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs1",
			CreationTimestamp: metav1.Now(),
		},
	}
	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs2",
			CreationTimestamp: metav1.Now(),
		},
	}
	rs3 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs3",
			CreationTimestamp: metav1.Now(),
		},
	}
	rsList := ReplicaSetsByCreationTimestamp{rs1, rs2, rs3}
	expectedLen := 3
	actualLen := rsList.Len()
	if actualLen != expectedLen {
		t.Errorf("Expected length to be %d, but got %d", expectedLen, actualLen)
	}
}

func TestReplicaSetsByCreationTimestamp_Swap(t *testing.T) {
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs1",
			CreationTimestamp: metav1.Now(),
		},
	}
	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs2",
			CreationTimestamp: metav1.Now(),
		},
	}
	r := ReplicaSetsByCreationTimestamp{rs1, rs2}
	r.Swap(0, 1)
	if r[0] != rs2 || r[1] != rs1 {
		t.Errorf("Swap failed, expected %v, got %v", []*appsv1.ReplicaSet{rs2, rs1}, r)
	}
}

func TestReplicaSetsByCreationTimestamp_Less(t *testing.T) {
	now := time.Now()
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs1",
			CreationTimestamp: metav1.NewTime(now),
		},
	}
	rs2 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "rs2",
			CreationTimestamp: metav1.NewTime(now.Add(time.Duration(-10) * time.Minute)),
		},
	}

	r := ReplicaSetsByCreationTimestamp{rs1, rs2}
	res := r.Less(0, 1)
	if res != false {
		t.Errorf("Expect false, but got true")
	}

	r = ReplicaSetsByCreationTimestamp{rs1, rs1}
	res = r.Less(0, 1)
	if res != false {
		t.Errorf("Expect false, but got true")
	}
}

func TestListReplicaSetsByDeployment(t *testing.T) {
	rs1 := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs1",
			Namespace: "test",
			Labels:    map[string]string{"key1": "value1"},
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        "test",
					Controller: pointer.Bool(true),
				},
			},
		},
	}
	err := fmt.Errorf("error")
	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		rs         *appsv1.ReplicaSet
		rsListFunc ReplicaSetListFunc
		wantErr    bool
		wantRS     bool
	}{
		{
			name: "get rs without error",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			rsListFunc: func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
				return []*appsv1.ReplicaSet{rs1}, nil
			},
			wantErr: false,
			wantRS:  true,
		},
		{
			name: "failed in LabelSelectorAsSelector",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "key1",
								Operator: "blah",
								Values:   []string{"value1"},
							},
						},
					},
				},
			},
			rsListFunc: func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
				return []*appsv1.ReplicaSet{rs1}, nil
			},
			wantErr: true,
			wantRS:  false,
		},
		{
			name: "failed in ReplicaSetListFunc",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			rsListFunc: func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
				return []*appsv1.ReplicaSet{rs1}, err
			},
			wantErr: true,
			wantRS:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rss, err := ListReplicaSetsByDeployment(tt.deployment, tt.rsListFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListReplicaSetsByDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantRS {
				assert.ObjectsAreEqual([]*appsv1.ReplicaSet{rs1}, rss)
			}
		})
	}
}

func TestListPodsByRS(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod1",
			Namespace: "test",
			Labels:    map[string]string{"key1": "value1"},
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        "test",
					Controller: pointer.Bool(true),
				},
			},
		},
	}
	err := fmt.Errorf("error")
	tests := []struct {
		name        string
		deployment  *appsv1.Deployment
		rsList      []*appsv1.ReplicaSet
		podListFunc PodListFunc
		wantErr     bool
		wantPod     bool
	}{
		{
			name: "get pods without error",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			rsList: []*appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rs1",
						Namespace: "test",
						Labels:    map[string]string{"key1": "value1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "test",
								Controller: pointer.Bool(true),
							},
						},
					},
				},
			},
			podListFunc: func(s string, selector labels.Selector) ([]*corev1.Pod, error) {
				return []*corev1.Pod{pod}, nil
			},
			wantErr: false,
			wantPod: true,
		},
		{
			name: "failed in LabelSelectorAsSelector",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      "key1",
								Operator: "blah",
								Values:   []string{"value1"},
							},
						},
					},
				},
			},
			rsList: []*appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rs1",
						Namespace: "test",
						Labels:    map[string]string{"key1": "value1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "test",
								Controller: pointer.Bool(true),
							},
						},
					},
				},
			},
			podListFunc: func(s string, selector labels.Selector) ([]*corev1.Pod, error) {
				return []*corev1.Pod{pod}, nil
			},
			wantErr: true,
			wantPod: false,
		},
		{
			name: "failed in ReplicaSetListFunc",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       "test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			rsList: []*appsv1.ReplicaSet{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "rs1",
						Namespace: "test",
						Labels:    map[string]string{"key1": "value1"},
						OwnerReferences: []metav1.OwnerReference{
							{
								UID:        "test",
								Controller: pointer.Bool(true),
							},
						},
					},
				},
			},
			podListFunc: func(s string, selector labels.Selector) ([]*corev1.Pod, error) {
				return []*corev1.Pod{pod}, err
			},
			wantErr: true,
			wantPod: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods, err := ListPodsByRS(tt.deployment, tt.rsList, tt.podListFunc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListPodsByRS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantPod {
				assert.ObjectsAreEqual([]*corev1.Pod{pod}, pods)
			}
		})
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#L339

func TestEqualIgnoreHash(t *testing.T) {
	tests := []struct {
		Name           string
		former, latter corev1.PodTemplateSpec
		expected       bool
	}{
		{
			"Same spec, same labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			true,
		},
		{
			"Same spec, only pod-template-hash label value is different",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			true,
		},
		{
			"Same spec, the former doesn't have pod-template-hash label",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			true,
		},
		{
			"Same spec, the label is different, the former doesn't have pod-template-hash label, same number of labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-2"}),
			false,
		},
		{
			"Same spec, the label is different, the latter doesn't have pod-template-hash label, same number of labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{"something": "else"}),
			false,
		},
		{
			"Same spec, the label is different, and the pod-template-hash label value is the same",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			false,
		},
		{
			"Different spec, same labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{"former": "value"}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{"latter": "value"}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			false,
		},
		{
			"Different spec, different pod-template-hash label value",
			generatePodTemplateSpec("foo-1", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo-2", "foo-node", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			false,
		},
		{
			"Different spec, the former doesn't have pod-template-hash label",
			generatePodTemplateSpec("foo-1", "foo-node-1", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo-2", "foo-node-2", map[string]string{}, map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			false,
		},
		{
			"Different spec, different labels",
			generatePodTemplateSpec("foo", "foo-node-1", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo", "foo-node-2", map[string]string{}, map[string]string{"nothing": "else"}),
			false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			runTest := func(t1, t2 *corev1.PodTemplateSpec, reversed bool) {
				reverseString := ""
				if reversed {
					reverseString = " (reverse order)"
				}
				// Run
				equal := EqualIgnoreHash(t1, t2)
				if equal != test.expected {
					t.Errorf("%q%s: expected %v", test.Name, reverseString, test.expected)
					return
				}
				if t1.Labels == nil || t2.Labels == nil {
					t.Errorf("%q%s: unexpected labels becomes nil", test.Name, reverseString)
				}
			}

			runTest(&test.former, &test.latter, false)
			// Test the same case in reverse order
			runTest(&test.latter, &test.former, true)
		})
	}
}

func TestGetNewReplicaSet(t *testing.T) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rs1",
			Namespace: "test",
			Labels:    map[string]string{"key1": "value1"},
			OwnerReferences: []metav1.OwnerReference{
				{
					UID:        "test",
					Controller: pointer.Bool(true),
				},
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "test",
					Labels:    map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value2"},
				},
			},
		},
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			UID:       "test",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"key1": "value1",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: "test",
					Labels:    map[string]string{appsv1.DefaultDeploymentUniqueLabelKey: "value1"},
				},
			},
		},
	}
	err := fmt.Errorf("error")

	t.Run("no error", func(t *testing.T) {
		rsListFunc := func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
			return []*appsv1.ReplicaSet{rs}, nil
		}
		gotRS, gotErr := GetNewReplicaSet(deployment, rsListFunc)
		if gotErr != nil {
			t.Errorf("Expect no error but got %v", err)
		}

		if gotRS != rs {
			t.Errorf("Got wrong rs %+v", gotRS)
		}
	})

	t.Run("have error", func(t *testing.T) {
		rsListFunc := func(namespace string, selector labels.Selector) ([]*appsv1.ReplicaSet, error) {
			return []*appsv1.ReplicaSet{rs}, err
		}
		gotRS, gotErr := GetNewReplicaSet(deployment, rsListFunc)
		if gotErr == nil {
			t.Errorf("Expect error but got %v", err)
		}

		if gotRS != nil {
			t.Errorf("Expect empty rs, but got %+v", gotRS)
		}
	})
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.22/pkg/controller/deployment/util/deployment_util_test.go#L432

func TestFindNewReplicaSet(t *testing.T) {
	now := metav1.Now()
	later := metav1.Time{Time: now.Add(time.Minute)}

	deployment := generateDeployment("nginx")
	newRS := generateRS(deployment)
	newRS.Labels[appsv1.DefaultDeploymentUniqueLabelKey] = "hash"
	newRS.CreationTimestamp = later

	newRSDup := generateRS(deployment)
	newRSDup.Labels[appsv1.DefaultDeploymentUniqueLabelKey] = "different-hash"
	newRSDup.CreationTimestamp = now

	oldDeployment := generateDeployment("nginx")
	oldDeployment.Spec.Template.Spec.Containers[0].Name = "nginx-old-1"
	oldRS := generateRS(oldDeployment)
	oldRS.Status.FullyLabeledReplicas = *(oldRS.Spec.Replicas)

	tests := []struct {
		Name       string
		deployment appsv1.Deployment
		rsList     []*appsv1.ReplicaSet
		expected   *appsv1.ReplicaSet
	}{
		{
			Name:       "Get new ReplicaSet with the same template as Deployment spec but different pod-template-hash value",
			deployment: deployment,
			rsList:     []*appsv1.ReplicaSet{&newRS, &oldRS},
			expected:   &newRS,
		},
		{
			Name:       "Get the oldest new ReplicaSet when there are more than one ReplicaSet with the same template",
			deployment: deployment,
			rsList:     []*appsv1.ReplicaSet{&newRS, &oldRS, &newRSDup},
			expected:   &newRSDup,
		},
		{
			Name:       "Get nil new ReplicaSet",
			deployment: deployment,
			rsList:     []*appsv1.ReplicaSet{&oldRS},
			expected:   nil,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if rs := FindNewReplicaSet(&test.deployment, test.rsList); !reflect.DeepEqual(rs, test.expected) {
				t.Errorf("In test case %q, expected %#v, got %#v", test.Name, test.expected, rs)
			}
		})
	}
}
