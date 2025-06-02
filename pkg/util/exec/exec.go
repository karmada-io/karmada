package exec

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"

	"k8s.io/klog/v2"
)

// Cmd abstracts over running a command somewhere, this is useful for testing
type Cmd interface {
	Run() error
	// Each entry should be of the form "key=value"
	SetEnv(...string)
	SetStdin(io.Reader)
	SetStdout(io.Writer)
	SetStderr(io.Writer)
}

// Cmder abstracts over creating commands
type Cmder interface {
	// command, args..., just like os/exec.Cmd
	Command(string, ...string) Cmd
}

// DefaultCmder is a LocalCmder instance used for convienience, packages
// originally using os/exec.Command can instead use pkg/kind/exec.Command
// which forwards to this instance
// TODO(bentheelder): swap this for testing
// TODO(bentheelder): consider not using a global for this :^)
var DefaultCmder = &LocalCmder{}

// Command is a convience wrapper over DefaultCmder.Command
func Command(command string, args ...string) Cmd {
	return DefaultCmder.Command(command, args...)
}

// CommandWithLogging is a convience wrapper over Command
// display any errors received by the executed command
func CommandWithLogging(command string, args ...string) error {
	cmd := Command(command, args...)
	output, err := CombinedOutputLines(cmd)
	if err != nil {
		// log error output if there was any
		for _, line := range output {
			klog.Error(line)
		}
	}
	return err
}

// CommandWithLogging is a convience wrapper over Command
// display any errors received by the executed command
func CommandWithLoggingNonBlocking(ctx context.Context, log func(string), command string, args ...string) error {
	cmd := Command(command, args...)
	var buff bytes.Buffer
	cmd.SetStdout(&buff)
	cmd.SetStderr(&buff)
	if err := cmd.Run(); err != nil {
		log(err.Error())
		return err
	}

	return nil
}

// CombinedOutputLines is like os/exec's cmd.CombinedOutput(),
// but over our Cmd interface, and instead of returning the byte buffer of
// stderr + stdout, it scans these for lines and returns a slice of output lines
func CombinedOutputLines(cmd Cmd) (lines []string, err error) {
	var buff bytes.Buffer
	cmd.SetStdout(&buff)
	cmd.SetStderr(&buff)
	err = cmd.Run()
	scanner := bufio.NewScanner(&buff)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, err
}

// InheritOutput sets cmd's output to write to the current process's stdout and stderr
func InheritOutput(cmd Cmd) {
	cmd.SetStderr(os.Stderr)
	cmd.SetStdout(os.Stdout)
}

// RunLoggingOutputOnFail runs the cmd, logging error output if Run returns an error
func RunLoggingOutputOnFail(cmd Cmd) error {
	var buff bytes.Buffer
	cmd.SetStdout(&buff)
	cmd.SetStderr(&buff)
	err := cmd.Run()
	if err != nil {
		klog.Error("failed with:")
		scanner := bufio.NewScanner(&buff)
		for scanner.Scan() {
			klog.Error(scanner.Text())
		}
	}
	return err
}
