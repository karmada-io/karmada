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

package karmadactl

import (
	"os"
	"os/exec"
	"testing"
)

// runKarmadactlCmd runs a karmadactl command with the given arguments and returns stdout, stderr, exit code, and error.
func runKarmadactlCmd(t testing.TB, args ...string) (string, string, int, error) {
	t.Helper()
	karmadactlPath := getKarmadactlPath()
	cmd := exec.Command(karmadactlPath, args...)
	stdout, err := cmd.Output()
	stderr := ""
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr = string(exitError.Stderr)
			return string(stdout), stderr, exitError.ExitCode(), err
		}
		return string(stdout), stderr, -1, err
	}
	return string(stdout), stderr, 0, nil
}

// getKarmadactlPath returns the path to the karmadactl binary.
func getKarmadactlPath() string {
	kubeadmPath := os.Getenv("KARMADACTL_PATH")
	if len(kubeadmPath) == 0 {
		panic("the environment variable KARMADACTL_PATH must point to the karmadactl binary path")
	}
	return kubeadmPath
}

// RunCmd runs a command and returns stdout, stderr, exit code, and error.
func RunCmd(cmdPath string, args ...string) (string, string, int, error) {
	cmd := exec.Command(cmdPath, args...)
	stdout, err := cmd.Output()
	stderr := ""
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			stderr = string(exitError.Stderr)
			return string(stdout), stderr, exitError.ExitCode(), err
		}
		return string(stdout), stderr, -1, err
	}
	return string(stdout), stderr, 0, nil
}
