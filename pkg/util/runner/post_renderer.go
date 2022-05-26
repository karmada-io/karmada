/*
Copyright 2021 The Flux authors

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

package runner

import (
	"bytes"

	"helm.sh/helm/v3/pkg/postrender"
)

// combinedPostRenderer, a collection of Helm PostRenders which are
// invoked in the order of insertion.
type combinedPostRenderer struct {
	renderers []postrender.PostRenderer
}

func newCombinedPostRenderer() combinedPostRenderer {
	return combinedPostRenderer{
		renderers: make([]postrender.PostRenderer, 0),
	}
}

func (c *combinedPostRenderer) addRenderer(renderer postrender.PostRenderer) {
	c.renderers = append(c.renderers, renderer)
}

func (c *combinedPostRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	var result *bytes.Buffer = renderedManifests
	for _, renderer := range c.renderers {
		result, err = renderer.Run(result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
