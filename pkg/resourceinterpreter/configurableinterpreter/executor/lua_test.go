package executor

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func Test_interpreterImpl_GetReplicas(t *testing.T) {
	type args struct {
		obj runtime.Object
	}
	tests := []struct {
		name         string
		script       string
		args         args
		wantReplica  int32
		wantRequires *workv1alpha2.ReplicaRequirements
		wantEnabled  bool
		wantErr      bool
	}{
		{
			name: "Test GetReplica",
			script: `function GetReplicas(desiredObj)
						replica = desiredObj.spec.replicas
						requirement = {}
						requirement.nodeClaim = {}
						requirement.nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector
						requirement.nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations
						requirement.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits
						return replica, requirement
					end`,
			args: args{
				obj: &appsv1.Deployment{
					Spec: appsv1.DeploymentSpec{
						Replicas: pointer.Int32(1),
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								NodeSelector: map[string]string{
									"foo": "bar",
								},
								Tolerations: []corev1.Toleration{
									{Key: "foo", Operator: corev1.TolerationOpExists},
								},
								Containers: []corev1.Container{
									{
										Resources: corev1.ResourceRequirements{
											Limits: corev1.ResourceList{
												corev1.ResourceCPU: resource.MustParse("100m"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantEnabled: true,
			wantReplica: 1,
			wantRequires: &workv1alpha2.ReplicaRequirements{
				NodeClaim: &workv1alpha2.NodeClaim{
					NodeSelector: map[string]string{
						"foo": "bar",
					},
					Tolerations: []corev1.Toleration{
						{Key: "foo", Operator: corev1.TolerationOpExists},
					},
				},
				ResourceRequest: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.ReplicaResourceRequirement{LuaScript: tt.script}

			obj, err := helper.ToUnstructured(tt.args.obj)
			if err != nil {
				t.Fatal(err)
			}

			gotReplica, gotRequires, gotEnabled, err := exec.GetReplicas(rule, obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetReplicas() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotReplica != tt.wantReplica {
				t.Errorf("GetReplicas() gotReplica = %v, want %v", gotReplica, tt.wantReplica)
			}
			if !reflect.DeepEqual(gotRequires, tt.wantRequires) {
				t.Errorf("GetReplicas() gotRequires = %v, want %v", gotRequires, tt.wantRequires)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("GetReplicas() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_ReviseReplica(t *testing.T) {
	type args struct {
		object  *unstructured.Unstructured
		replica int64
	}
	tests := []struct {
		name        string
		script      string
		args        args
		wantRevised *unstructured.Unstructured
		wantEnabled bool
		wantErr     bool
	}{
		{
			name: "test ReviseReplica",
			script: `function ReviseReplica(desiredObj, desiredReplica)
						desiredObj.spec.replicas = desiredReplica
						return desiredObj
					end`,
			args: args{
				object: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"spec": map[string]interface{}{
						"replicas": 0,
					}},
				},
				replica: 1,
			},
			wantEnabled: true,
			wantRevised: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"spec": map[string]interface{}{
					"replicas": int64(1),
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.ReplicaRevision{LuaScript: tt.script}

			obj, err := helper.ToUnstructured(tt.args.object)
			if err != nil {
				t.Fatal(err)
			}

			gotRevised, gotEnabled, err := exec.ReviseReplica(rule, obj, tt.args.replica)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReviseReplica() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			wantRevised, err := helper.ToUnstructured(tt.wantRevised)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gotRevised, wantRevised) {
				t.Errorf("ReviseReplica() gotRevised = %v, want %v", gotRevised, wantRevised)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("ReviseReplica() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_Retain(t *testing.T) {
	type args struct {
		desired  *unstructured.Unstructured
		observed *unstructured.Unstructured
	}
	tests := []struct {
		name         string
		script       string
		args         args
		wantRetained *unstructured.Unstructured
		wantEnabled  bool
		wantErr      bool
	}{
		{
			name: "test Retain",
			script: `function Retain(desiredObj, observedObj) 
						desiredObj.foo = observedObj.foo 
						return desiredObj
					end`,
			args: args{
				desired: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"foo":        "2",
					"bar":        "2",
				}},
				observed: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"foo":        "1",
					"bar":        "1",
				}},
			},
			wantEnabled: true,
			wantRetained: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"foo":        "1",
				"bar":        "2",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.LocalValueRetention{LuaScript: tt.script}
			gotRetained, gotEnabled, err := exec.Retain(rule, tt.args.desired, tt.args.observed)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRetained, tt.wantRetained) {
				t.Errorf("Retain() gotRetained = %v, want %v", gotRetained, tt.wantRetained)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("Retain() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_AggregateStatus(t *testing.T) {
	type args struct {
		obj   *unstructured.Unstructured
		items []workv1alpha2.AggregatedStatusItem
	}
	tests := []struct {
		name           string
		script         string
		args           args
		wantAggregated *unstructured.Unstructured
		wantEnabled    bool
		wantErr        bool
	}{
		{
			name: "test AggregateStatus",
			script: `function AggregateStatus(desiredObj, statusItems) 
						for i = 1, #statusItems do    
							desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + statusItems[i].status.readyReplicas 
						end    
						return desiredObj
					end`,
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"status": map[string]interface{}{
						"readyReplicas": 1,
					},
				}},
				items: []workv1alpha2.AggregatedStatusItem{
					{
						Status: &runtime.RawExtension{Raw: []byte(`{"readyReplicas": 1}`)},
					},
				},
			},
			wantEnabled: true,
			wantAggregated: &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"status": map[string]interface{}{
					"readyReplicas": int64(2),
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.StatusAggregation{LuaScript: tt.script}
			gotRetained, gotEnabled, err := exec.AggregateStatus(rule, tt.args.obj, tt.args.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRetained, tt.wantAggregated) {
				t.Errorf("Retain() gotAggregated = %v, want %v", gotRetained, tt.wantAggregated)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("Retain() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_InterpretHealth(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name        string
		script      string
		args        args
		wantHealthy bool
		wantEnabled bool
		wantErr     bool
	}{
		{
			name: "test InterpretHealth",
			script: `function InterpretHealth(observedObj)
						return (observedObj.status.updatedReplicas == observedObj.spec.replicas) and (observedObj.metadata.generation == observedObj.status.observedGeneration)
					end`,
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"generation": 1,
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
					"status": map[string]interface{}{
						"updatedReplicas":    1,
						"observedGeneration": 1,
					},
				}},
			},
			wantEnabled: true,
			wantHealthy: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.HealthInterpretation{LuaScript: tt.script}
			gotHealthy, gotEnabled, err := exec.InterpretHealth(rule, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotHealthy, tt.wantHealthy) {
				t.Errorf("Retain() gotHealthy = %v, want %v", gotHealthy, tt.wantHealthy)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("Retain() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_ReflectStatus(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name        string
		script      string
		args        args
		wantStatus  *runtime.RawExtension
		wantEnabled bool
		wantErr     bool
	}{
		{
			name: "test ReflectStatus",
			script: `function ReflectStatus (observedObj)
						if observedObj.status == nil then
							return nil
						end
						return observedObj.status
					end`,
			args: args{
				obj: &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"status": map[string]interface{}{
						"updatedReplicas": 1,
					},
				}},
			},
			wantEnabled: true,
			wantStatus:  &runtime.RawExtension{Raw: []byte(`{"updatedReplicas":1}`)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.StatusReflection{LuaScript: tt.script}
			gotStatus, gotEnabled, err := exec.ReflectStatus(rule, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotStatus, tt.wantStatus) {
				t.Errorf("Retain() goStatus = %v, want %v", gotStatus, tt.wantStatus)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("Retain() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}

func Test_interpreterImpl_GetDependencies(t *testing.T) {
	type args struct {
		obj *appsv1.Deployment
	}
	tests := []struct {
		name             string
		script           string
		args             args
		wantDependencies []configv1alpha1.DependentObjectReference
		wantEnabled      bool
		wantErr          bool
	}{
		{
			name: "test GetDependencies",
			script: `function GetDependencies(desiredObj)
						dependentSas = {}
						refs = {}
						if desiredObj.spec.template.spec.serviceAccountName ~= '' and desiredObj.spec.template.spec.serviceAccountName ~= 'default' then
							dependentSas[desiredObj.spec.template.spec.serviceAccountName] = true
						end
						local idx = 1
						for key, value in pairs(dependentSas) do    
							dependObj = {}    
							dependObj.apiVersion = 'v1'   
							dependObj.kind = 'ServiceAccount'    
							dependObj.name = key    
							dependObj.namespace = desiredObj.metadata.namespace    
							refs[idx] = {} 
							refs[idx] = dependObj    
							idx = idx + 1
						end
						return refs
				    end`,
			args: args{
				obj: &appsv1.Deployment{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Deployment"},
					ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								ServiceAccountName: "test",
							},
						},
					},
				},
			},
			wantEnabled: true,
			wantDependencies: []configv1alpha1.DependentObjectReference{
				{APIVersion: "v1", Kind: "ServiceAccount", Namespace: "test", Name: "test"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewLuaExecutor()
			rule := &configv1alpha1.DependencyInterpretation{LuaScript: tt.script}
			obj, err := helper.ToUnstructured(tt.args.obj)
			if err != nil {
				t.Fatal(err)
			}
			goDependencies, gotEnabled, err := exec.GetDependencies(rule, obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("Retain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(goDependencies, tt.wantDependencies) {
				t.Errorf("Retain() goDependencies = %v, want %v", goDependencies, tt.wantDependencies)
			}
			if gotEnabled != tt.wantEnabled {
				t.Errorf("Retain() gotEnabled = %v, want %v", gotEnabled, tt.wantEnabled)
			}
		})
	}
}
