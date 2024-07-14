/*
Copyright 2023 The Karmada Authors.

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

package framework

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"syscall"
	"time"

	"k8s.io/client-go/tools/clientcmd"
	uexec "k8s.io/utils/exec"
)

// KarmadactlBuilder is a builder to run karmadactl.
type KarmadactlBuilder struct {
	cmd     *exec.Cmd
	timeout <-chan time.Time
}

// TestKubeconfig contains the information of a test kubeconfig.
type TestKubeconfig struct {
	KubeConfig     string
	KarmadaContext string
	KarmadactlPath string
	Namespace      string
}

// NewKarmadactlCommand creates a new KarmadactlBuilder.
func NewKarmadactlCommand(kubeConfig, karmadaContext, karmadactlPath, namespace string, timeout time.Duration, args ...string) *KarmadactlBuilder {
	builder := new(KarmadactlBuilder)
	tk := NewTestKubeconfig(kubeConfig, karmadaContext, karmadactlPath, namespace)
	builder.cmd = tk.KarmadactlCmd(args...)
	builder.timeout = time.After(timeout)
	return builder
}

// NewTestKubeconfig creates a new TestKubeconfig.
func NewTestKubeconfig(kubeConfig, karmadaContext, karmadactlpath, namespace string) *TestKubeconfig {
	return &TestKubeconfig{
		KubeConfig:     kubeConfig,
		KarmadaContext: karmadaContext,
		KarmadactlPath: karmadactlpath,
		Namespace:      namespace,
	}
}

// KarmadactlCmd runs the karmadactl executable through the wrapper script.
func (tk *TestKubeconfig) KarmadactlCmd(args ...string) *exec.Cmd {
	var defaultArgs []string

	if tk.KubeConfig != "" {
		defaultArgs = append(defaultArgs, "--"+clientcmd.RecommendedConfigPathFlag+"="+tk.KubeConfig)

		// Reference the KubeContext
		if tk.KarmadaContext != "" {
			defaultArgs = append(defaultArgs, "--karmada-context"+"="+tk.KarmadaContext)
		}
	}
	if tk.Namespace != "" {
		defaultArgs = append(defaultArgs, fmt.Sprintf("--namespace=%s", tk.Namespace))
	}
	karmadactlArgs := append(defaultArgs, args...)

	//We allow users to specify path to karmadactl
	cmd := exec.Command(tk.KarmadactlPath, karmadactlArgs...) //nolint:gosec

	//caller will invoke this and wait on it.
	return cmd
}

func isTimeout(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	}
	return false
}

// ExecOrDie executes the karmadactl executable and returns the stdout.
func (k *KarmadactlBuilder) ExecOrDie() (string, error) {
	str, err := k.exec()
	if isTimeout(err) {
		return "", fmt.Errorf("timed out waiting for %v", k.cmd)
	}
	return str, err
}

// exec runs the karmadactl executable and returns the stdout.
func (k *KarmadactlBuilder) exec() (string, error) {
	stdOut, _, err := k.execWithFullOutput()
	return stdOut, err
}

// execWithFullOutput runs the karmadactl executable, and returns the stdout and stderr.
func (k *KarmadactlBuilder) execWithFullOutput() (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := k.cmd
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("error starting %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v", cmd, cmd.Stdout, cmd.Stderr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()
	select {
	case err := <-errCh:
		if err != nil {
			var rc = 127
			if ee, ok := err.(*exec.ExitError); ok {
				rc = ee.Sys().(syscall.WaitStatus).ExitStatus()
			}
			return stdout.String(), stderr.String(), uexec.CodeExitError{
				Err:  fmt.Errorf("error running %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v", cmd, cmd.Stdout, cmd.Stderr, err),
				Code: rc,
			}
		}
	case <-k.timeout:
		err := k.cmd.Process.Kill()
		if err != nil {
			return "", "", fmt.Errorf("after execution timeout, error killing %v:\nCommand stdout:\n%v\nstderr:\n%v\nerror:\n%v", cmd, cmd.Stdout, cmd.Stderr, err)
		}
		return "", "", fmt.Errorf("timed out waiting for command %v:\nCommand stdout:\n%v\nstderr:\n%v", cmd, cmd.Stdout, cmd.Stderr)
	}
	return stdout.String(), stderr.String(), nil
}
