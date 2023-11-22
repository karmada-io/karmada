/*
Copyright 2022 The Karmada Authors.

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

package interpret

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/util"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/genericresource"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

func (o *Options) completeExecute(f util.Factory) []error {
	var errs []error
	if o.DesiredFile != "" {
		o.DesiredResult = f.NewBuilder().
			Unstructured().
			FilenameParam(false, &resource.FilenameOptions{Filenames: []string{o.DesiredFile}}).
			RequireObject(true).
			Local().
			Do()
		errs = append(errs, o.DesiredResult.Err())
	}

	if o.ObservedFile != "" {
		o.ObservedResult = f.NewBuilder().
			Unstructured().
			FilenameParam(false, &resource.FilenameOptions{Filenames: []string{o.ObservedFile}}).
			RequireObject(true).
			Local().
			Do()
		errs = append(errs, o.ObservedResult.Err())
	}

	if len(o.StatusFile) > 0 {
		o.StatusResult = genericresource.NewBuilder().
			Constructor(func() interface{} { return &workv1alpha2.AggregatedStatusItem{} }).
			Filename(false, o.StatusFile).
			Do()
		errs = append(errs, o.StatusResult.Err())
	}
	return errs
}

func (o *Options) runExecute() error {
	if o.Operation == "" {
		return fmt.Errorf("operation is not set for executing")
	}

	customizations, err := o.getCustomizationObject()
	if err != nil {
		return fmt.Errorf("fail to get customization object: %v", err)
	}

	desired, err := getUnstructuredObjectFromResult(o.DesiredResult)
	if err != nil {
		return fmt.Errorf("fail to get desired object: %v", err)
	}

	observed, err := getUnstructuredObjectFromResult(o.ObservedResult)
	if err != nil {
		return fmt.Errorf("fail to get observed object: %v", err)
	}

	status, err := o.getAggregatedStatusItems()
	if err != nil {
		return fmt.Errorf("fail to get status items: %v", err)
	}

	args := interpreter.RuleArgs{
		Desired:  desired,
		Observed: observed,
		Status:   status,
		Replica:  int64(o.DesiredReplica),
	}

	configurableInterpreter := declarative.NewConfigurableInterpreter(nil)
	configurableInterpreter.LoadConfig(customizations)

	r := o.Rules.GetByOperation(o.Operation)
	if r == nil {
		// Shall never occur, because we validate it before.
		return fmt.Errorf("operation %s is not supported. Use one of: %s", o.Operation, strings.Join(o.Rules.Names(), ", "))
	}
	result := r.Run(configurableInterpreter, args)
	printExecuteResult(o.Out, o.ErrOut, r.Name(), result)
	return nil
}

func printExecuteResult(w, errOut io.Writer, name string, result *interpreter.RuleResult) {
	if result.Err != nil {
		fmt.Fprintf(errOut, "Execute %s error: %v\n", name, result.Err)
		return
	}

	for i, res := range result.Results {
		func() {
			fmt.Fprintln(w, "---")
			fmt.Fprintf(w, "# [%v/%v] %s:\n", i+1, len(result.Results), res.Name)
			if err := printObjectYaml(w, res.Value); err != nil {
				fmt.Fprintf(errOut, "ERROR: %v\n", err)
			}
		}()
	}
}

// MarshalJSON doesn't work for yaml encoder, so unstructured.Unstructured and runtime.RawExtension objects
// will be encoded into unexpected data.
// Example1:
//
//	  &unstructured.Unstructured{
//	  	Object: map[string]interface{}{
//				"foo": "bar"
//	  	},
//	  }
//
// will be encoded into:
//
//	Object:
//	  foo: bar
//
// Example2:
//
//	&runtime.RawExtension{
//		Raw: []byte("{}"),
//	}
//
// will be encoded into:
//
//	raw:
//	  - 123
//	  - 125
//
// Inspired from https://github.com/kubernetes/kubernetes/blob/8fb423bfabe0d53934cc94c154c7da2dc3ce1332/staging/src/k8s.io/kubectl/pkg/cmd/get/get.go#L781-L786
// we convert it to map[string]interface{} by json, then encode the converted object to yaml.
func printObjectYaml(w io.Writer, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	var converted interface{}
	err = json.Unmarshal(data, &converted)
	if err != nil {
		return err
	}

	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	return encoder.Encode(converted)
}
