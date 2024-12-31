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

package completion

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

const defaultBoilerPlate = `
# Copyright 2024 The Karmada Authors.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
`

var (
	completionLong = templates.LongDesc(i18n.T(`
		Output shell completion code for the specified shell (bash, zsh, fish).
		The shell code must be evaluated to provide interactive
		completion of %[1]s commands. This can be done by sourcing it from
		the .bash_profile.

		Note for zsh users: zsh completions are only supported in versions of zsh >= 5.2.`))

	completionExample = templates.Examples(i18n.T(`
		# Installing bash completion on Linux
		## If bash-completion is not installed on Linux, install the 'bash-completion' package
		    1. apt-get install bash-completion
		    2. source /usr/share/bash-completion/bash_completion
		## Load the %[1]s completion code for bash into the current shell
		    source <(%[1]s completion bash)
		## Or, write bash completion code to a file and source it from .bash_profile
		    1. %[1]s completion bash > ~/.kube/completion.bash.inc
		    2. echo "source '$HOME/.kube/completion.bash.inc'" >> $HOME/.bash_profile
		    3. source $HOME/.bash_profile

		# Load the %[1]s completion code for zsh into the current shell
		    source <(%[1]s completion zsh)
		# Set the %[1]s completion code for zsh to autoload on startup
		    %[1]s completion zsh > "${fpath[1]}/%[1]s"

		# Load the %[1]s completion code for fish into the current shell
		    %[1]s completion fish | source
		# To load completions for each session, execute once:
		    %[1]s completion fish > ~/.config/fish/completions/%[1]s.fish`))
)

var (
	completionShells = map[string]func(out io.Writer, boilerPlate string, cmd *cobra.Command) error{
		"bash": runCompletionBash,
		"zsh":  runCompletionZsh,
		"fish": runCompletionFish,
	}
)

// NewCmdCompletion creates the `completion` command
func NewCmdCompletion(parentCommand string, out io.Writer, boilerPlate string) *cobra.Command {
	var shells []string
	for s := range completionShells {
		shells = append(shells, s)
	}

	cmd := &cobra.Command{
		Use:                   "completion SHELL",
		DisableFlagsInUseLine: true,
		Short:                 "Output shell completion code for the specified shell (bash, zsh, fish)",
		Long:                  fmt.Sprintf(completionLong, parentCommand),
		Example:               fmt.Sprintf(completionExample, parentCommand),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(RunCompletion(out, boilerPlate, cmd, args))
		},
		ValidArgs: shells,
	}

	return cmd
}

// RunCompletion checks given arguments and executes command
func RunCompletion(out io.Writer, boilerPlate string, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmdutil.UsageErrorf(cmd, "Shell not specified.")
	}
	if len(args) > 1 {
		return cmdutil.UsageErrorf(cmd, "Too many arguments. Expected only the shell type.")
	}
	run, found := completionShells[args[0]]
	if !found {
		return cmdutil.UsageErrorf(cmd, "Unsupported shell type %q.", args[0])
	}

	return run(out, boilerPlate, cmd.Parent())
}

func runCompletionBash(out io.Writer, boilerPlate string, cmd *cobra.Command) error {
	if len(boilerPlate) == 0 {
		boilerPlate = defaultBoilerPlate
	}
	if _, err := out.Write([]byte(boilerPlate)); err != nil {
		return err
	}

	return cmd.GenBashCompletionV2(out, true)
}

func runCompletionZsh(out io.Writer, boilerPlate string, cmd *cobra.Command) error {
	zshHead := fmt.Sprintf("#compdef %[1]s\ncompdef _%[1]s %[1]s\n", cmd.Name())
	if _, err := out.Write([]byte(zshHead)); err != nil {
		return err
	}

	if len(boilerPlate) == 0 {
		boilerPlate = defaultBoilerPlate
	}
	if _, err := out.Write([]byte(boilerPlate)); err != nil {
		return err
	}

	return cmd.GenZshCompletion(out)
}

func runCompletionFish(out io.Writer, boilerPlate string, cmd *cobra.Command) error {
	if len(boilerPlate) == 0 {
		boilerPlate = defaultBoilerPlate
	}
	if _, err := out.Write([]byte(boilerPlate)); err != nil {
		return err
	}
	return cmd.GenFishCompletion(out, true)
}
