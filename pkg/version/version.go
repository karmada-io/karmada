/*
Copyright 2021 The Karmada Authors.

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

package version

import (
	"fmt"
	"runtime"
)

// Info contains versioning information.
type Info struct {
	GitVersion     string `json:"gitVersion"`
	GitCommit      string `json:"gitCommit"`
	GitShortCommit string `json:"gitShortCommit"`
	GitTreeState   string `json:"gitTreeState"`
	BuildDate      string `json:"buildDate"`
	GoVersion      string `json:"goVersion"`
	Compiler       string `json:"compiler"`
	Platform       string `json:"platform"`
}

// String returns a Go-syntax representation of the Info.
func (info Info) String() string {
	return fmt.Sprintf("%#v", info)
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	return Info{
		GitVersion:     gitVersion,
		GitShortCommit: gitShortCommit,
		GitCommit:      gitCommit,
		GitTreeState:   gitTreeState,
		BuildDate:      buildDate,
		GoVersion:      runtime.Version(),
		Compiler:       runtime.Compiler,
		Platform:       fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
