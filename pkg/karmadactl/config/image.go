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
)

var (
	imagesLong = templates.LongDesc(`
		Shows information about the images required for Karmada deployment.

		This command lists all the images needed to deploy and run Karmada, including
		images for the API server, controller manager, scheduler, and other components.`)

	imagesExamples = templates.Examples(`
		# List all required images from the specified private image registry
		%[1]s config image list

		# Use a specific registry for Karmada images
		%[1]s config image list --private-image-registry registry.cn-hangzhou.aliyuncs.com/google_containers`)
)

// NewCmdConfigImage creates a new image command
func NewCmdConfigImage(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "image",
		Short:                 "Shows information about the images required for Karmada components.",
		Long:                  imagesLong,
		Example:               fmt.Sprintf(imagesExamples, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.AddCommand(NewCmdConfigImagesList(parentCommand))

	return cmd
}
