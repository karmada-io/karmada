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

package addons

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/addons/install"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	addonsLong = templates.LongDesc(`
		Enable or disable a Karmada addon.

		These addons are currently supported: 
		
		1. karmada-descheduler
		2. karmada-metrics-adapter
		3. karmada-scheduler-estimator
		4. karmada-search`)

	addonsExamples = templates.Examples(`
	# Enable or disable Karmada addons to the karmada-host cluster
	%[1]s addons enable karmada-search
	`)
)

func init() {
	install.Install()
}

// NewCmdAddons enable or disable Karmada addons on karmada-host cluster
func NewCmdAddons(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "addons",
		Short:   "Enable or disable a Karmada addon",
		Long:    addonsLong,
		Example: fmt.Sprintf(addonsExamples, parentCommand),
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}

	addonsParentCommand := fmt.Sprintf("%s %s", parentCommand, "addons")
	cmd.AddCommand(NewCmdAddonsList(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsEnable(addonsParentCommand))
	cmd.AddCommand(NewCmdAddonsDisable(addonsParentCommand))

	return cmd
}
