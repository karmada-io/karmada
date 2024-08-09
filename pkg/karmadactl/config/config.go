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

package config

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

var (
	configLong = templates.LongDesc(`
		Manage Karmada configurations.

		The 'config' command allows you to manage the configurations required for
		deploying and running Karmada. This includes listing, setting, and modifying
		various configurations such as image registries, versions, and other settings.`)

	configExamples = templates.Examples(`
		# List all images required for deploying Karmada
		%[1]s config image list

		# Use a specific registry for Karmada images
		%[1]s config image list --private-image-registry registry.cn-hangzhou.aliyuncs.com/google_containers`)
)

// NewCmdConfig creates a new config command
func NewCmdConfig(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Short:   "Manage Karmada configurations",
		Long:    configLong,
		Example: fmt.Sprintf(configExamples, parentCommand),
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupAdvancedCommands,
		},
	}

	configParentCommand := fmt.Sprintf("%s %s", parentCommand, "config")
	cmd.AddCommand(NewCmdConfigImage(configParentCommand))

	return cmd
}
