/*
Copyright 2024 The Karmada Authors.

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

package create

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectlcreate "k8s.io/kubectl/pkg/cmd/create"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	createLong = templates.LongDesc(`
		Create a resource from a file or from stdin.

		JSON and YAML formats are accepted.`)

	createExample = templates.Examples(`
		# Create a pod using the data in pod.json
		%[1]s create -f ./pod.json

		# Create a pod based on the JSON passed into stdin
		cat pod.json | %[1]s create -f -

		# Edit the data in registry.yaml in JSON then create the resource using the edited data
		%[1]s create -f registry.yaml --edit -o json`)
)

// NewCmdCreate returns new initialized instance of create sub command
func NewCmdCreate(f util.Factory, parentCommnd string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectlcreate.NewCmdCreate(f, ioStreams)
	cmd.Long = fmt.Sprintf(createLong, parentCommnd)
	cmd.Example = fmt.Sprintf(createExample, parentCommnd)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupBasic,
	}
	return cmd
}
