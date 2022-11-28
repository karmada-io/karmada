package resourceinterpretercustomization

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

func Test_validateCustomizationRule(t *testing.T) {
	type args struct {
		oldRules *configv1alpha1.ResourceInterpreterCustomization
		newRules *configv1alpha1.ResourceInterpreterCustomization
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "the different Kind of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "bar",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: " the different APIVersion of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v2",
							Kind:       "kind",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "the same InterpreterOperation(Retention) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							Retention: &configv1alpha1.LocalValueRetention{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(ReplicaResource) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(ReplicaRevision) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(StatusReflection) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							StatusReflection: &configv1alpha1.StatusReflection{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							StatusReflection: &configv1alpha1.StatusReflection{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(StatusAggregation) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(HealthInterpretation) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "the same InterpreterOperation(DependencyInterpretation) of ResourceInterpreterCustomization",
			args: args{
				oldRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							DependencyInterpretation: &configv1alpha1.DependencyInterpretation{LuaScript: "LuaScript"},
						},
					},
				},
				newRules: &configv1alpha1.ResourceInterpreterCustomization{
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
						Target: configv1alpha1.CustomizationTarget{
							APIVersion: "foo/v1",
							Kind:       "kind",
						},
						Customizations: configv1alpha1.CustomizationRules{
							DependencyInterpretation: &configv1alpha1.DependencyInterpretation{LuaScript: "LuaScript"},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCustomizationRule(tt.args.oldRules, tt.args.newRules); (err != nil) != tt.wantErr {
				t.Errorf("validateCustomizationRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateResourceInterpreterCustomizations(t *testing.T) {
	type args struct {
		customization  *configv1alpha1.ResourceInterpreterCustomization
		customizations *configv1alpha1.ResourceInterpreterCustomizationList
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "the same name of ResourceInterpreterCustomization",
			args: args{
				customization: &configv1alpha1.ResourceInterpreterCustomization{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
					Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{Target: configv1alpha1.CustomizationTarget{
						APIVersion: "foo/v1",
						Kind:       "kind",
					}, Customizations: configv1alpha1.CustomizationRules{Retention: &configv1alpha1.LocalValueRetention{LuaScript: `function Retain(desiredObj, observedObj) end`}}}},
				customizations: &configv1alpha1.ResourceInterpreterCustomizationList{
					Items: []configv1alpha1.ResourceInterpreterCustomization{
						{ObjectMeta: metav1.ObjectMeta{Name: "test"},
							Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{Target: configv1alpha1.CustomizationTarget{
								APIVersion: "foo/v1",
								Kind:       "kind",
							}, Customizations: configv1alpha1.CustomizationRules{Retention: &configv1alpha1.LocalValueRetention{LuaScript: "function Retain(desiredObj, observedObj) end"}}}}}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateResourceInterpreterCustomizations(tt.args.customization, tt.args.customizations); (err != nil) != tt.wantErr {
				t.Errorf("validateResourceInterpreterCustomizations() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkCustomizationsRule(t *testing.T) {
	type args struct {
		customization *configv1alpha1.ResourceInterpreterCustomization
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "correct lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						Retention: &configv1alpha1.LocalValueRetention{LuaScript: `
function Retain(desiredObj, observedObj)
    desiredObj.spec.fieldFoo = observedObj.spec.fieldFoo
    return desiredObj
end
`},
						ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: `
function GetReplicas(desiredObj)                                                            
    nodeClaim = {}                                                                          
    resourceRequest = {}                                                                    
    result = {}                                                                             
                                                                                            
    result.replica = desiredObj.spec.replicas                                               
    result.resourceRequest = desiredObj.spec.template.spec.containers[0].resources.limits   
                                                                                            
    nodeClaim.nodeSelector = desiredObj.spec.template.spec.nodeSelector                     
    nodeClaim.tolerations = desiredObj.spec.template.spec.tolerations                       
    result.nodeClaim = nodeClaim                                                            
                                                                                            
    return result                                                                           
end
`},
						ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: `
function ReviseReplica(desiredObj, desiredReplica)         
    desiredObj.spec.replicas = desiredReplica              
    return desiredObj                                      
end                                                        
`},
						StatusReflection: &configv1alpha1.StatusReflection{LuaScript: `
function ReflectStatus(observedObj)                         
    status = {}                                             
    status.readyReplicas = observedObj.status.observedObj   
    return status                                           
end                                                         

`},
						StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: `
function AggregateStatus(desiredObj, statusItems)                                                     
    for i = 1, #items do                                                                              
        desiredObj.status.readyReplicas = desiredObj.status.readyReplicas + items[i].readyReplicas    
    end                                                                                               
    return desiredObj                                                                                 
end                                                                                                   

`},
						HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: `
function InterpretHealth(observedObj)                                             
    if observedObj.status.readyReplicas == observedObj.spec.replicas then         
        return true                                                               
    end                                                                           
end                                                                               
`},
						DependencyInterpretation: &configv1alpha1.DependencyInterpretation{LuaScript: `
function GetDependencies(desiredObj)                                                                     
    dependencies = {}                                                                                    
    if desiredObj.spec.serviceAccountName ~= "" and desiredObj.spec.serviceAccountName ~= "default" then 
        dependency = {}                                                                                  
        dependency.apiVersion = "v1"                                                                     
        dependency.kind = "ServiceAccount"                                                               
        dependency.name = desiredObj.spec.serviceAccountName                                             
        dependency.namespace = desiredObj.namespace                                                      
        dependencies[0] = {}                                                                             
        dependencies[0] = dependency                                                                     
    end                                                                                                  
    return dependencies                                                                                  
end                                                                                                      

`}},
				},
			},
			},
			wantErr: false,
		},
		{
			name: "Retention contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						Retention: &configv1alpha1.LocalValueRetention{LuaScript: `function Retain(desiredObj, observedObj)`},
					},
				},
			}},
			wantErr: true,
		},
		{
			name: "ReplicaResource contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						ReplicaResource: &configv1alpha1.ReplicaResourceRequirement{LuaScript: `function GetReplicas(desiredObj)`},
					},
				},
			}},
			wantErr: true,
		},
		{
			name: "ReplicaRevision contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						ReplicaRevision: &configv1alpha1.ReplicaRevision{LuaScript: `function ReviseReplica(desiredObj, desiredReplica)`}},
				},
			}},
			wantErr: true,
		},
		{
			name: "StatusReflection contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						StatusReflection: &configv1alpha1.StatusReflection{LuaScript: `function ReflectStatus(observedObj)`}},
				},
			}},
			wantErr: true,
		},
		{
			name: "StatusAggregation contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						StatusAggregation: &configv1alpha1.StatusAggregation{LuaScript: `function AggregateStatus(desiredObj, statusItems)`}},
				},
			}},
			wantErr: true,
		},
		{
			name: "HealthInterpretation contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						HealthInterpretation: &configv1alpha1.HealthInterpretation{LuaScript: `function InterpretHealth(observedObj)`}},
				},
			}},
			wantErr: true,
		},
		{
			name: "DependencyInterpretation contains the wrong lua script",
			args: args{customization: &configv1alpha1.ResourceInterpreterCustomization{
				Spec: configv1alpha1.ResourceInterpreterCustomizationSpec{
					Customizations: configv1alpha1.CustomizationRules{
						DependencyInterpretation: &configv1alpha1.DependencyInterpretation{LuaScript: `function GetDependencies(desiredObj)`}},
				},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkCustomizationsRule(tt.args.customization); (err != nil) != tt.wantErr {
				t.Errorf("checkCustomizationsRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
