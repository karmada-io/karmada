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

package edit

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectledit "k8s.io/kubectl/pkg/cmd/edit"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	editExample = templates.Examples(i18n.T(`
		# Edit the service named 'registry'
		%[1]s edit svc/registry

		# Use an alternative editor
		KUBE_EDITOR="nano" %[1]s edit svc/registry

		# Edit the job 'myjob' in JSON using the v1 API format
		%[1]s edit job.v1.batch/myjob -o json

		# Edit the deployment 'mydeployment' in YAML and save the modified config in its annotation
		%[1]s edit deployment/mydeployment -o yaml --save-config

		# Edit the 'status' subresource for the 'mydeployment' deployment
		%[1]s edit deployment mydeployment --subresource='status'`))
)

// NewCmdEdit returns new initialized instance of edit sub command
func NewCmdEdit(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectledit.NewCmdEdit(f, ioStreams)
	cmd.Example = fmt.Sprintf(editExample, parentCommand)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupBasic,
	}
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	return cmd
}
