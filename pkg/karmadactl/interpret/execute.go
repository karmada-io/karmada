package interpret

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/cmd/util"
)

func (o *Options) completeExecute(_ util.Factory, _ *cobra.Command, _ []string) []error {
	return nil
}

func (o *Options) runExecute() error {
	return fmt.Errorf("not implement")
}
