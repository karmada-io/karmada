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

package delete

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubectldelete "k8s.io/kubectl/pkg/cmd/delete"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
)

var (
	deleteLong = templates.LongDesc(`
		Delete resources by file names, stdin, resources and names, or by resources and label selector.

		JSON and YAML formats are accepted. Only one type of argument may be specified: file names,
		resources and names, or resources and label selector.

		Some resources, such as pods, support graceful deletion. These resources define a default period
		before they are forcibly terminated (the grace period) but you may override that value with
		the --grace-period flag, or pass --now to set a grace-period of 1. Because these resources often
		represent entities in the cluster, deletion may not be acknowledged immediately. If the node
		hosting a pod is down or cannot reach the API server, termination may take significantly longer
		than the grace period. To force delete a resource, you must specify the --force flag.
		Note: only a subset of resources support graceful deletion. In absence of the support,
		the --grace-period flag is ignored.

		IMPORTANT: Force deleting pods does not wait for confirmation that the pod's processes have been
		terminated, which can leave those processes running until the node detects the deletion and
		completes graceful deletion. If your processes use shared storage or talk to a remote API and
		depend on the name of the pod to identify themselves, force deleting those pods may result in
		multiple processes running on different machines using the same identification which may lead
		to data corruption or inconsistency. Only force delete pods when you are sure the pod is
		terminated, or if your application can tolerate multiple copies of the same pod running at once.
		Also, if you force delete pods, the scheduler may place new pods on those nodes before the node
		has released those resources and causing those pods to be evicted immediately.

		Note that the delete command does NOT do resource version checks, so if someone submits an
		update to a resource right when you submit a delete, their update will be lost along with the
		rest of the resource.

		After a CustomResourceDefinition is deleted, invalidation of discovery cache may take up
		to 6 hours. If you don't want to wait, you might want to run "[1]%s api-resources" to refresh
		the discovery cache.`)

	deleteExample = templates.Examples(`
		# Delete a propagationpolicy using the type and name specified in propagationpolicy.json
		[1]%s delete -f ./propagationpolicy.json

		# Delete resources from a directory containing kustomization.yaml - e.g. dir/kustomization.yaml
		[1]%s delete -k dir

		# Delete resources from all files that end with '.json'
		[1]%s delete -f '*.json'

		# Delete a propagationpolicy based on the type and name in the JSON passed into stdin
		cat propagationpolicy.json | [1]%s delete -f -

		# Delete propagationpolicies and services with same names "baz" and "foo"
		[1]%s delete propagationpolicy,service baz foo

		# Delete propagationpolicies and services with label name=myLabel
		[1]%s delete propagationpolicies,services -l name=myLabel

		# Delete a propagationpolicy with minimal delay
		[1]%s delete propagationpolicy foo --now

		# Force delete a propagationpolicy on a dead node
		[1]%s delete propagationpolicy foo --force

		# Delete all propagationpolicies
		[1]%s delete propagationpolicies --all`)
)

// NewCmdDelete returns new initialized instance of delete sub command
func NewCmdDelete(f util.Factory, parentCommand string, ioStreams genericiooptions.IOStreams) *cobra.Command {
	cmd := kubectldelete.NewCmdDelete(f, ioStreams)
	cmd.Long = fmt.Sprintf(deleteLong, parentCommand)
	cmd.Example = fmt.Sprintf(deleteExample, parentCommand)
	cmd.Annotations = map[string]string{
		util.TagCommandGroup: util.GroupBasic,
	}
	options.AddKubeConfigFlags(cmd.Flags())
	options.AddNamespaceFlag(cmd.Flags())

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	utilcomp.RegisterCompletionFuncForNamespaceFlag(cmd, f)
	return cmd
}
