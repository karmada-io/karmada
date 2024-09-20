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

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	configInit "github.com/karmada-io/karmada/pkg/karmadactl/config/init"
)

var (
	listExample = templates.Examples(`
	# List Karmada all required images from the specified private image registry
	%[1]s list --private-image-registry=myregistry.com`)
)

// NewCmdConfigImagesList creates a new list command
func NewCmdConfigImagesList(parentCommand string) *cobra.Command {
	opts := configInit.CommandConfigImageOption{
		InitOption: kubernetes.NewDefaultCommandInitOption(),
	}
	cmd := &cobra.Command{
		Use:                   "list",
		Short:                 "List the images required for Karmada",
		Long:                  "List the images required for Karmada from the specified private image registry.",
		Example:               fmt.Sprintf(listExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := opts.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.InitOption.ImageRegistry, "private-image-registry", "", "", "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.")
	cmd.Flags().StringVarP(&opts.InitOption.KubeImageMirrorCountry, "kube-image-country", "", "", "The country code of the image registry, such 'global', 'cn'.")

	return cmd
}
