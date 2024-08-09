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
	"os"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
)

// CommandConfigImageOption options for components
type CommandConfigImageOption struct {
	PrivateImageRegistry string
	KubeImageCountry     string
	InitOption           *kubernetes.CommandInitOption
}

// Complete sets up the CommandConfigImageOption with the provided options.
func (o *CommandConfigImageOption) Complete() error {
	o.InitOption = kubernetes.NewDefaultCommandInitOption()
	if o.PrivateImageRegistry != "" {
		o.InitOption.ImageRegistry = o.PrivateImageRegistry
	}
	if o.KubeImageCountry != "" {
		o.InitOption.KubeImageMirrorCountry = o.KubeImageCountry
	}
	return nil
}

// Run generates and prints the required control plane images.
func (o *CommandConfigImageOption) Run() error {
	imageList := o.InitOption.GenerateControlPlaneImages()
	for _, image := range imageList {
		_, err := fmt.Fprintf(os.Stdout, "%s\n", image)
		if err != nil {
			return err
		}
	}
	return nil
}
