package helper

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestGenEventRef(t *testing.T) {
	tests := []struct {
		name    string
		obj     *unstructured.Unstructured
		want    *corev1.ObjectReference
		wantErr bool
	}{
		{
			name: "has metadata.uid",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
						"uid":  "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid but has resourcetemplate.karmada.io/uid annontation",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":        "demo-deployment",
						"annotations": map[string]interface{}{"resourcetemplate.karmada.io/uid": "9249d2e7-3169-4c5f-be82-163bd80aa3cf"},
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			want: &corev1.ObjectReference{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
				Name:       "demo-deployment",
				UID:        "9249d2e7-3169-4c5f-be82-163bd80aa3cf",
			},
			wantErr: false,
		},
		{
			name: "missing metadata.uid and metadata.annotations",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "demo-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GenEventRef(tt.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenEventRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("GenEventRef() = %v, want %v", actual, tt.want)
			}
		})
	}
}

func TestGetWorksByBindingNamespaceName(t *testing.T) {
	bindingName := "test-deployment"
	bindingNamespace := "default"

	works := []*workv1alpha1.Work{
		newWork(bindingNamespace, bindingName, "member-1"),
		newWork(bindingNamespace, bindingName, "member-2"),
		newWork(bindingNamespace, bindingName, "member-3"),
	}

	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
	restMapper.Add(workv1alpha1.SchemeGroupVersion.WithKind("Work"), meta.RESTScopeRoot)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion(workv1alpha1.GroupVersion), &workv1alpha1.Work{}, &workv1alpha1.WorkList{})

	client := clientfake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).Build()

	for i := range works {
		err := client.Create(context.TODO(), works[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	workList, err := GetWorksByBindingNamespaceName(client, bindingNamespace, bindingName)
	if err != nil {
		t.Fatal(err)
		return
	}

	if !isWorkEqual(workList.Items, works) {
		t.Errorf("Expectedworks: %v, but got: %v", workList.Items, works)
	}
}

func newWork(bindingNamespace string, bindingName string, clusterName string) *workv1alpha1.Work {
	esname, err := names.GenerateExecutionSpaceName(clusterName)

	if err != nil {
		return nil
	}

	referenceKey := names.GenerateBindingReferenceKey(bindingNamespace, bindingName)
	ls := labels.Set{workv1alpha2.ResourceBindingReferenceKey: referenceKey}

	annotations := make(map[string]string)
	annotations[workv1alpha2.ResourceBindingNameAnnotationKey] = bindingName
	annotations[workv1alpha2.ResourceBindingNamespaceAnnotationKey] = bindingNamespace

	return &workv1alpha1.Work{
		TypeMeta:   metav1.TypeMeta{APIVersion: workv1alpha1.SchemeGroupVersion.String(), Kind: "Work"},
		ObjectMeta: metav1.ObjectMeta{Name: "test" + clusterName, Namespace: esname, Labels: ls, Annotations: annotations},
	}
}

func isWorkEqual(expectedWorks []workv1alpha1.Work, works []*workv1alpha1.Work) (res bool) {
	if len(expectedWorks) != len(works) {
		return false
	}

	for _, work := range works {
		flag := false
		for _, expectedWork := range expectedWorks {
			if work.Namespace == expectedWork.Namespace && work.Name == expectedWork.Name {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	return true
}
