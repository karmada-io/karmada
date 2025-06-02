/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/05/18
 * @Desc    : local exec
 */

package exec

import (
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"strings"

	"k8s.io/klog/v2"
)

// LocalCmd wraps os/exec.Cmd, implementing the kind/pkg/exec.Cmd interface
type LocalCmd struct {
	*osexec.Cmd
}

var _ Cmd = &LocalCmd{}

// LocalCmder is a factory for LocalCmd, implementing Cmder
type LocalCmder struct{}

var _ Cmder = &LocalCmder{}

// Command returns a new exec.Cmd backed by Cmd
func (c *LocalCmder) Command(name string, arg ...string) Cmd {
	return &LocalCmd{
		Cmd: osexec.Command(name, arg...),
	}
}

// SetEnv sets env
func (cmd *LocalCmd) SetEnv(env ...string) {
	cmd.Env = env
}

// SetStdin sets stdin
func (cmd *LocalCmd) SetStdin(r io.Reader) {
	cmd.Stdin = r
}

// SetStdout set stdout
func (cmd *LocalCmd) SetStdout(w io.Writer) {
	cmd.Stdout = w
}

// SetStderr sets stderr
func (cmd *LocalCmd) SetStderr(w io.Writer) {
	cmd.Stderr = w
}

// Run runs
func (cmd *LocalCmd) Run() error {
	klog.Infof("Running: %v %v", cmd.Path, cmd.Args)
	return cmd.Cmd.Run()
}

func ExecuteCommand(command string, args ...string) (string, error) {
	cmd := osexec.Command(command, args...)
	out, err := cmd.CombinedOutput()
	cmdArgs := strings.Join(cmd.Args, " ")
	//log.Debugf("Command %q returned %q\n", cmdArgs, out)
	if err != nil {
		return "", fmt.Errorf("%+v: command %q exited with %q", err, cmdArgs, out)
	}

	// TODO: strings.Builder?
	return strings.TrimSpace(string(out)), nil
}

func ExecForeground(command string, args ...string) (int, error) {
	cmd := osexec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	cmdArgs := strings.Join(cmd.Args, " ")

	var cmdErr error
	var exitCode int

	if err != nil {
		cmdErr = fmt.Errorf("external command %q exited with an error: %v", cmdArgs, err)

		if exitError, ok := err.(*osexec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			cmdErr = fmt.Errorf("failed to get exit code for external command %q", cmdArgs)
		}
	}

	return exitCode, cmdErr
}
