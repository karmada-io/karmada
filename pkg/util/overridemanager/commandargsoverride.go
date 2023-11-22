/*
Copyright 2021 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package overridemanager

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util"
)

const (
	// CommandString command string
	CommandString = "command"
	// ArgsString args string
	ArgsString = "args"
)

// buildCommandArgsPatches build JSON patches for the resource object according to override declaration.
func buildCommandArgsPatches(target string, rawObj *unstructured.Unstructured, commandRunOverrider *policyv1alpha1.CommandArgsOverrider) ([]overrideOption, error) {
	switch rawObj.GetKind() {
	case util.PodKind:
		return buildCommandArgsPatchesWithPath(target, "spec/containers", rawObj, commandRunOverrider)
	case util.ReplicaSetKind:
		fallthrough
	case util.DeploymentKind:
		fallthrough
	case util.DaemonSetKind:
		fallthrough
	case util.JobKind:
		fallthrough
	case util.StatefulSetKind:
		return buildCommandArgsPatchesWithPath(target, "spec/template/spec/containers", rawObj, commandRunOverrider)
	}
	return nil, nil
}
func buildCommandArgsPatchesWithPath(target string, specContainersPath string, rawObj *unstructured.Unstructured, commandRunOverrider *policyv1alpha1.CommandArgsOverrider) ([]overrideOption, error) {
	patches := make([]overrideOption, 0)
	containers, ok, err := unstructured.NestedSlice(rawObj.Object, strings.Split(specContainersPath, pathSplit)...)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieves path(%s) from rawObj, error: %v", specContainersPath, err)
	}
	if !ok || len(containers) == 0 {
		return nil, nil
	}
	klog.V(4).Infof("buildCommandArgsPatchesWithPath containers info (%+v)", containers)
	for index, container := range containers {
		if container.(map[string]interface{})["name"] == commandRunOverrider.ContainerName {
			commandArgsPath := fmt.Sprintf("/%s/%d/%s", specContainersPath, index, target)
			commandArgsValue := make([]string, 0)
			var patch overrideOption
			// if target is nil, to add new [target]
			if container.(map[string]interface{})[target] == nil {
				patch, _ = acquireAddOverrideOption(commandArgsPath, commandRunOverrider)
			} else {
				for _, val := range container.(map[string]interface{})[target].([]interface{}) {
					commandArgsValue = append(commandArgsValue, fmt.Sprintf("%s", val))
				}
				patch, _ = acquireReplaceOverrideOption(commandArgsPath, commandArgsValue, commandRunOverrider)
			}

			klog.V(4).Infof("[buildCommandArgsPatchesWithPath] containers patch info (%+v)", patch)
			patches = append(patches, patch)
		}
	}
	return patches, nil
}

func acquireAddOverrideOption(commandArgsPath string, commandOverrider *policyv1alpha1.CommandArgsOverrider) (overrideOption, error) {
	if !strings.HasPrefix(commandArgsPath, pathSplit) {
		return overrideOption{}, fmt.Errorf("internal error: [acquireCommandOverrideOption] commandRunPath should be start with / character")
	}
	newCommandArgs, err := overrideCommandArgs([]string{}, commandOverrider)
	if err != nil {
		return overrideOption{}, err
	}
	return overrideOption{
		Op:    string(policyv1alpha1.OverriderOpAdd),
		Path:  commandArgsPath,
		Value: newCommandArgs,
	}, nil
}

func acquireReplaceOverrideOption(commandArgsPath string, commandArgsValue []string, commandOverrider *policyv1alpha1.CommandArgsOverrider) (overrideOption, error) {
	if !strings.HasPrefix(commandArgsPath, pathSplit) {
		return overrideOption{}, fmt.Errorf("internal error: [acquireCommandOverrideOption] commandRunPath should be start with / character")
	}

	newCommandArgs, err := overrideCommandArgs(commandArgsValue, commandOverrider)
	if err != nil {
		return overrideOption{}, err
	}

	return overrideOption{
		Op:    string(policyv1alpha1.OverriderOpReplace),
		Path:  commandArgsPath,
		Value: newCommandArgs,
	}, nil
}

func overrideCommandArgs(curCommandArgs []string, commandArgsOverrider *policyv1alpha1.CommandArgsOverrider) ([]string, error) {
	var newCommandArgs []string
	switch commandArgsOverrider.Operator {
	case policyv1alpha1.OverriderOpAdd:
		newCommandArgs = append(curCommandArgs, commandArgsOverrider.Value...)
	case policyv1alpha1.OverriderOpRemove:
		newCommandArgs = commandArgsRemove(curCommandArgs, commandArgsOverrider.Value)
	default:
		newCommandArgs = curCommandArgs
		klog.V(4).Infof("[overrideCommandArgs], op: %s , op not supported, ignored.", policyv1alpha1.OverriderOpRemove)
	}
	return newCommandArgs, nil
}

func commandArgsRemove(curCommandArgs []string, removeValues []string) []string {
	newCommandArgs := make([]string, 0, len(curCommandArgs))
	currentSet := sets.NewString(removeValues...)
	for _, val := range curCommandArgs {
		if !currentSet.Has(val) {
			newCommandArgs = append(newCommandArgs, val)
		}
	}
	return newCommandArgs
}
