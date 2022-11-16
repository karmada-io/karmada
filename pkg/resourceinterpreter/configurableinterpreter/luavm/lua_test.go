package luavm

import (
	"encoding/json"
	"reflect"
	"testing"

	lua "github.com/yuin/gopher-lua"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/util/helper"
)

func TestGetReplicas(t *testing.T) {
	var replicas int32 = 1
	vm := VM{UseOpenLibs: false}
	tests := []struct {
		name         string
		deploy       *appsv1.Deployment
		luaScript    string
		expected     bool
		wantReplica  int32
		wantRequires *workv1alpha2.ReplicaRequirements
	}{
		{
			name: "Test GetReplica",
			deploy: &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{},
							},
						},
					},
				},
			},
			expected: true,
			luaScript: ` function GetReplicas(desiredObj) 
							nodeClaim = {} 
							resourceRequest = {} 
							result = {} 
							replica = desiredObj.spec.replicas 
							result.resourceRequest = desiredObj.spec.template.spec.containers[1].resources.limits 
							nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector 
							nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations 
							result.nodeClaim = {} 
							result.nodeClaim = nil
							return replica, {} 
						end`,
			wantReplica:  1,
			wantRequires: &workv1alpha2.ReplicaRequirements{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toUnstructured, _ := helper.ToUnstructured(tt.deploy)
			replicas, requires, err := vm.GetReplicas(toUnstructured, tt.luaScript)
			if err != nil {
				t.Errorf(err.Error())
			}
			if !reflect.DeepEqual(replicas, tt.wantReplica) {
				t.Errorf("GetReplicas() got = %v, want %v", replicas, tt.wantReplica)
			}
			if !reflect.DeepEqual(requires, tt.wantRequires) {
				t.Errorf("GetReplicas() got = %v, want %v", requires, tt.wantRequires)
			}
		})
	}
}

func TestReviseDeploymentReplica(t *testing.T) {
	tests := []struct {
		name        string
		object      *unstructured.Unstructured
		replica     int32
		expected    *unstructured.Unstructured
		expectError bool
		luaScript   string
	}{
		{
			name: "Test ReviseDeploymentReplica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 1,
					},
				},
			},
			replica:     3,
			expectError: true,
			luaScript: `function ReviseReplica(desiredObj, desiredReplica)
							desiredObj.spec.replicas = desiredReplica  
							return desiredObj
                           end`,
		},
		{
			name: "revise deployment replica",
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			replica: 3,
			expected: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectError: false,
			luaScript: `function ReviseReplica(desiredObj, desiredReplica)  
							desiredObj.spec.replicas = desiredReplica	
							return desiredObj 
						end`,
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := vm.ReviseReplica(tt.object, int64(tt.replica), tt.luaScript)
			if err != nil {
				t.Errorf(err.Error())
			}
			deploy := &appsv1.Deployment{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.UnstructuredContent(), deploy)
			if err == nil && *deploy.Spec.Replicas == tt.replica {
				t.Log("Success Test")
			}
			if err != nil {
				t.Errorf(err.Error())
			}
		})
	}
}

func TestAggregateDeploymentStatus(t *testing.T) {
	statusMap := map[string]interface{}{
		"replicas":            0,
		"readyReplicas":       1,
		"updatedReplicas":     0,
		"availableReplicas":   0,
		"unavailableReplicas": 0,
	}
	raw, _ := helper.BuildStatusRawExtension(statusMap)
	aggregatedStatusItems := []workv1alpha2.AggregatedStatusItem{
		{ClusterName: "member1", Status: raw, Applied: true},
		{ClusterName: "member2", Status: raw, Applied: true},
	}

	oldDeploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
	}
	oldDeploy.Status = appsv1.DeploymentStatus{
		Replicas: 0, ReadyReplicas: 1, UpdatedReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: 0}

	newDeploy := &appsv1.Deployment{Status: appsv1.DeploymentStatus{Replicas: 0, ReadyReplicas: 3, UpdatedReplicas: 0, AvailableReplicas: 0, UnavailableReplicas: 0}}
	oldObj, _ := helper.ToUnstructured(oldDeploy)
	newObj, _ := helper.ToUnstructured(newDeploy)

	tests := []struct {
		name                  string
		curObj                *unstructured.Unstructured
		aggregatedStatusItems []workv1alpha2.AggregatedStatusItem
		expectedObj           *unstructured.Unstructured
		luaScript             string
	}{
		{
			name:                  "Test AggregateDeploymentStatus",
			curObj:                oldObj,
			aggregatedStatusItems: aggregatedStatusItems,
			expectedObj:           newObj,
			luaScript: `function AggregateStatus(desiredObj, statusItems) 
										for i = 1, #statusItems do    
											desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + statusItems[i].status.readyReplicas 
										end    
										return desiredObj
									end`,
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		actualObj, _ := vm.AggregateStatus(tt.curObj, tt.aggregatedStatusItems, tt.luaScript)
		actualDeploy := appsv1.DeploymentStatus{}
		err := helper.ConvertToTypedObject(actualObj.Object["status"], &actualDeploy)
		if err != nil {
			t.Error(err.Error())
		}
		expectDeploy := appsv1.DeploymentStatus{}
		err = helper.ConvertToTypedObject(tt.expectedObj.Object["status"], &expectDeploy)
		if err != nil {
			t.Error(err.Error())
		}
		if !reflect.DeepEqual(expectDeploy, actualDeploy) {
			t.Errorf("AggregateStatus() got = %v, want %v", actualDeploy, expectDeploy)
		}
	}
}

func TestHealthDeploymentStatus(t *testing.T) {
	var cnt int32 = 2
	newDeploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &cnt,
		},

		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 1, Replicas: 2, ReadyReplicas: 2, UpdatedReplicas: 2, AvailableReplicas: 2}}
	newObj, _ := helper.ToUnstructured(newDeploy)

	tests := []struct {
		name        string
		curObj      *unstructured.Unstructured
		expectedObj bool
		luaScript   string
	}{
		{
			name:        "Test HealthDeploymentStatus",
			curObj:      newObj,
			expectedObj: true,
			luaScript: `function InterpretHealth(observedObj)
							return (observedObj.status.updatedReplicas == observedObj.spec.replicas) and (observedObj.metadata.generation == observedObj.status.observedGeneration)
                        end `,
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		flag, err := vm.InterpretHealth(tt.curObj, tt.luaScript)

		if err != nil {
			t.Error(err.Error())
		}
		if !reflect.DeepEqual(flag, tt.expectedObj) {
			t.Errorf("AggregateStatus() got = %v, want %v", flag, tt.expectedObj)
		}
	}
}

func TestRetainDeployment(t *testing.T) {
	tests := []struct {
		name        string
		desiredObj  *unstructured.Unstructured
		observedObj *unstructured.Unstructured
		expectError bool
		luaScript   string
	}{
		{
			name: "Test RetainDeployment1",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": 2,
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(2),
					},
				},
			},
			expectError: true,
			luaScript:   "function Retain(desiredObj, observedObj)\n  desiredObj = observedObj\n  return desiredObj\n end",
		},
		{
			name: "Test RetainDeployment2",
			desiredObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			observedObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name": "fake-deployment",
					},
					"spec": map[string]interface{}{
						"replicas": int64(1),
					},
				},
			},
			expectError: false,
			luaScript: `function Retain(desiredObj, observedObj) 
							desiredObj = observedObj 
							return desiredObj
						end`,
		},
	}
	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := vm.Retain(tt.desiredObj, tt.observedObj, tt.luaScript)
			if err != nil {
				t.Errorf(err.Error())
			}
			if !reflect.DeepEqual(res.UnstructuredContent(), tt.observedObj.Object) {
				t.Errorf("Retain() got = %v, want %v", res.UnstructuredContent(), tt.observedObj.Object)
			}
		})
	}
}

func TestStatusReflection(t *testing.T) {
	testMap := map[string]interface{}{"key": "value"}
	wantRawExtension, _ := helper.BuildStatusRawExtension(testMap)
	type args struct {
		object *unstructured.Unstructured
	}
	tests := []struct {
		name      string
		args      args
		want      *runtime.RawExtension
		wantErr   bool
		luaScript string
	}{
		{
			"Test StatusReflection",
			args{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"status": testMap,
					},
				},
			},
			wantRawExtension,
			false,
			`function ReflectStatus (observedObj)
						if observedObj.status == nil then	
							return nil   
						end    
					return observedObj.status
					end`,
		},
	}

	vm := VM{UseOpenLibs: false}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vm.ReflectStatus(tt.args.object, tt.luaScript)
			if (err != nil) != tt.wantErr {
				t.Errorf("reflectWholeStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reflectWholeStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetDeployPodDependencies(t *testing.T) {
	newDeploy := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "test",
				},
			},
		},
	}
	newObj, _ := helper.ToUnstructured(&newDeploy)
	expect := make([]configv1alpha1.DependentObjectReference, 1)
	expect[0] = configv1alpha1.DependentObjectReference{
		APIVersion: "v1",
		Kind:       "ServiceAccount",
		Namespace:  "test",
		Name:       "test",
	}
	tests := []struct {
		name      string
		curObj    *unstructured.Unstructured
		luaScript string
		want      []configv1alpha1.DependentObjectReference
	}{
		{
			name:   "Get GetDeployPodDependencies",
			curObj: newObj,
			luaScript: `function GetDependencies(desiredObj)
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
			want: expect,
		},
	}

	vm := VM{UseOpenLibs: false}

	for _, tt := range tests {
		res, err := vm.GetDependencies(tt.curObj, tt.luaScript)
		if err != nil {
			t.Errorf("GetDependencies err %v", err)
		}
		if !reflect.DeepEqual(res, tt.want) {
			t.Errorf("GetDependencies() got = %v, want %v", res, tt.want)
		}
	}
}

func Test_decodeValue(t *testing.T) {
	L := lua.NewState()

	type args struct {
		value interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    lua.LValue
		wantErr bool
	}{
		{
			name: "nil",
			args: args{
				value: nil,
			},
			want: lua.LNil,
		},
		{
			name: "nil pointer",
			args: args{
				value: (*struct{})(nil),
			},
			want: lua.LNil,
		},
		{
			name: "int pointer",
			args: args{
				value: pointer.Int(1),
			},
			want: lua.LNumber(1),
		},
		{
			name: "int",
			args: args{
				value: 1,
			},
			want: lua.LNumber(1),
		},
		{
			name: "uint",
			args: args{
				value: uint(1),
			},
			want: lua.LNumber(1),
		},
		{
			name: "float",
			args: args{
				value: 1.0,
			},
			want: lua.LNumber(1),
		},
		{
			name: "bool",
			args: args{
				value: true,
			},
			want: lua.LBool(true),
		},
		{
			name: "string",
			args: args{
				value: "foo",
			},
			want: lua.LString("foo"),
		},
		{
			name: "json number",
			args: args{
				value: json.Number("1"),
			},
			want: lua.LString("1"),
		},
		{
			name: "slice",
			args: args{
				value: []string{"foo", "bar"},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LString("foo"))
				v.Append(lua.LString("bar"))
				return v
			}(),
		},
		{
			name: "slice pointer",
			args: args{
				value: &[]string{"foo", "bar"},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LString("foo"))
				v.Append(lua.LString("bar"))
				return v
			}(),
		},
		{
			name: "struct",
			args: args{
				value: struct {
					Foo string
				}{
					Foo: "foo",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("Foo", lua.LString("foo"))
				return v
			}(),
		},
		{
			name: "struct pointer",
			args: args{
				value: &struct {
					Foo string
				}{
					Foo: "foo",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("Foo", lua.LString("foo"))
				return v
			}(),
		},
		{
			name: "[]interface{}",
			args: args{
				value: []interface{}{1, 2},
			},
			want: func() lua.LValue {
				v := L.CreateTable(2, 0)
				v.Append(lua.LNumber(1))
				v.Append(lua.LNumber(2))
				return v
			}(),
		},
		{
			name: "map[string]interface{}",
			args: args{
				value: map[string]interface{}{
					"foo": "foo1",
				},
			},
			want: func() lua.LValue {
				v := L.CreateTable(0, 1)
				v.RawSetString("foo", lua.LString("foo1"))
				return v
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeValue(L, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeValue() got = %v, want %v", got, tt.want)
			}
		})
	}
}
