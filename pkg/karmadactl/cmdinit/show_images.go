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

package cmdinit

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	configInit "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/show"
)

var (
	showImagesLong = templates.LongDesc(`
        Shows information about the images required for Karmada deployment.

        This command lists all the images needed to deploy and run Karmada, including
        images for the API server, controller manager, scheduler, and other components.`)

	showImagesExample = templates.Examples(`
        # List all required images
        %[1]s init show-images

        # List all required images from a specific private image registry
        %[1]s init show-images --private-image-registry registry.cn-hangzhou.aliyuncs.com/google_containers

        # List all required images with a specific country code for the image registry
        %[1]s init show-images --kube-image-country cn`)
)

// NewCmdShowImages creates a new show-images command
func NewCmdShowImages(parentCommand string) *cobra.Command {
	opts := configInit.CommandConfigImageOption{
		InitOption: kubernetes.NewDefaultCommandInitOption(),
	}
	cmd := &cobra.Command{
		Use:     "show-images",
		Short:   "List the images required for Karmada deployment",
		Long:    showImagesLong,
		Example: fmt.Sprintf(showImagesExample, parentCommand),
		RunE: func(_ *cobra.Command, _ []string) error {
			return opts.Run()
		},
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
	}

	cmd.Flags().StringVarP(&opts.InitOption.ImageRegistry, "private-image-registry", "", "", "Private image registry to pull images from. If set, all required images will be downloaded from it, useful for offline installation scenarios.")
	cmd.Flags().StringVarP(&opts.InitOption.KubeImageMirrorCountry, "kube-image-country", "", "", "The country code of the image registry, such as 'global' or 'cn'.")

	return cmd
}
