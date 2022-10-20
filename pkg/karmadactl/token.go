package karmadactl

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubeclient "k8s.io/client-go/kubernetes"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/cmd/get"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	tokenutil "github.com/karmada-io/karmada/pkg/karmadactl/util/bootstraptoken"
)

var (
	tokenLong = templates.LongDesc(`
	This command manages bootstrap tokens. It is optional and needed only for advanced use cases.

	In short, bootstrap tokens are used for establishing bidirectional trust between a client and a server.
	A bootstrap token can be used when a client (for example a member cluster that is about to join control plane) needs
	to trust the server it is talking to. Then a bootstrap token with the "signing" usage can be used.
	bootstrap tokens can also function as a way to allow short-lived authentication to the API Server
	(the token serves as a way for the API Server to trust the client), for example for doing the TLS Bootstrap.

	What is a bootstrap token more exactly?
	 - It is a Secret in the kube-system namespace of type "bootstrap.kubernetes.io/token".
	 - A bootstrap token must be of the form "[a-z0-9]{6}.[a-z0-9]{16}". The former part is the public token ID,
	   while the latter is the Token Secret and it must be kept private at all circumstances!
	 - The name of the Secret must be named "bootstrap-token-(token-id)".

	This command is same as 'kubeadm token', but it will create tokens that are used by member clusters.`)

	tokenExamples = templates.Examples(`
	# Create a token and print the full '%[1]s register' flag needed to join the cluster using the token.
	%[1]s token create --print-register-command
	`)
)

// NewCmdToken returns cobra.Command for token management
func NewCmdToken(karmadaConfig KarmadaConfig, parentCommand string, streams genericclioptions.IOStreams) *cobra.Command {
	opts := &CommandTokenOptions{
		parentCommand: parentCommand,
		TTL: &metav1.Duration{
			Duration: 0,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Manage bootstrap tokens",
		Long:    tokenLong,
		Example: fmt.Sprintf(tokenExamples, parentCommand),
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}

	cmd.AddCommand(NewCmdTokenCreate(karmadaConfig, streams.Out, opts))
	cmd.AddCommand(NewCmdTokenList(karmadaConfig, streams.Out, streams.ErrOut, opts))
	cmd.AddCommand(NewCmdTokenDelete(karmadaConfig, streams.Out, opts))

	return cmd
}

// CommandTokenOptions holds all command options for token
type CommandTokenOptions struct {
	// global flags
	options.GlobalCommandOptions

	TTL         *metav1.Duration
	Description string
	Groups      []string
	Usages      []string

	PrintRegisterCommand bool

	parentCommand string
}

// NewCmdTokenCreate returns cobra.Command to create token
func NewCmdTokenCreate(karmadaConfig KarmadaConfig, out io.Writer, tokenOpts *CommandTokenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create bootstrap tokens on the server",
		Long: templates.LongDesc(`
			This command will create a bootstrap token for you.
			You can specify the usages for this token, the "time to live" and an optional human friendly description.

			This should be a securely generated random token of the form "[a-z0-9]{6}.[a-z0-9]{16}".
		`),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(Cmd *cobra.Command, args []string) error {
			// Get control plane kube-apiserver client
			client, err := tokenOpts.getClientSet(karmadaConfig)
			if err != nil {
				return err
			}

			return tokenOpts.runCreateToken(out, client)
		},
		Args: cobra.NoArgs,
	}

	tokenOpts.GlobalCommandOptions.AddFlags(cmd.Flags())

	if tokenOpts.KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			tokenOpts.KubeConfig = env
		} else {
			tokenOpts.KubeConfig = defaultKubeConfig
		}
	}

	cmd.Flags().BoolVar(&tokenOpts.PrintRegisterCommand, "print-register-command", false, fmt.Sprintf("Instead of printing only the token, print the full '%s join' flag needed to join the member cluster using the token.", tokenOpts.parentCommand))
	cmd.Flags().DurationVar(&tokenOpts.TTL.Duration, "ttl", tokenutil.DefaultTokenDuration, "The duration before the token is automatically deleted (e.g. 1s, 2m, 3h). If set to '0', the token will never expire")
	cmd.Flags().StringSliceVar(&tokenOpts.Usages, "usages", tokenutil.DefaultUsages, fmt.Sprintf("Describes the ways in which this token can be used. You can pass --usages multiple times or provide a comma separated list of options. Valid options: [%s]", strings.Join(bootstrapapi.KnownTokenUsages, ",")))
	cmd.Flags().StringSliceVar(&tokenOpts.Groups, "groups", tokenutil.DefaultGroups, fmt.Sprintf("Extra groups that this token will authenticate as when used for authentication. Must match %q", bootstrapapi.BootstrapGroupPattern))
	cmd.Flags().StringVar(&tokenOpts.Description, "description", tokenOpts.Description, "A human friendly description of how this token is used.")

	return cmd
}

// NewCmdTokenList returns cobra.Command to list tokens
func NewCmdTokenList(karmadaConfig KarmadaConfig, out io.Writer, errW io.Writer, tokenOpts *CommandTokenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bootstrap tokens on the server",
		Long:  "This command will list all bootstrap tokens for you.",
		RunE: func(tokenCmd *cobra.Command, args []string) error {
			// Get control plane kube-apiserver client
			client, err := tokenOpts.getClientSet(karmadaConfig)
			if err != nil {
				return err
			}

			return tokenOpts.runListTokens(client, out, errW)
		},
		Args: cobra.NoArgs,
	}

	tokenOpts.GlobalCommandOptions.AddFlags(cmd.Flags())

	if tokenOpts.KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			tokenOpts.KubeConfig = env
		} else {
			tokenOpts.KubeConfig = defaultKubeConfig
		}
	}

	return cmd
}

// NewCmdTokenDelete returns cobra.Command to delete tokens
func NewCmdTokenDelete(karmadaConfig KarmadaConfig, out io.Writer, tokenOpts *CommandTokenOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "delete [token-value] ...",
		DisableFlagsInUseLine: true,
		Short:                 "Delete bootstrap tokens on the server",
		Long: templates.LongDesc(`
			This command will delete a list of bootstrap tokens for you.

			The [token-value] is the full Token of the form "[a-z0-9]{6}.[a-z0-9]{16}" or the
			Token ID of the form "[a-z0-9]{6}" to delete.
		`),
		RunE: func(tokenCmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing subcommand; 'token delete' is missing token of form %q", bootstrapapi.BootstrapTokenIDPattern)
			}

			// Get control plane kube-apiserver client
			client, err := tokenOpts.getClientSet(karmadaConfig)
			if err != nil {
				return err
			}

			return tokenOpts.runDeleteTokens(out, client, args)
		},
	}

	tokenOpts.GlobalCommandOptions.AddFlags(cmd.Flags())

	if tokenOpts.KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			tokenOpts.KubeConfig = env
		} else {
			tokenOpts.KubeConfig = defaultKubeConfig
		}
	}

	return cmd
}

// runCreateToken generates a new bootstrap token and stores it as a secret on the server.
func (o *CommandTokenOptions) runCreateToken(out io.Writer, client kubeclient.Interface) error {
	klog.V(1).Infoln("[token] creating token")
	bootstrapToken, err := tokenutil.GenerateRandomBootstrapToken(o.TTL, o.Description, o.Groups, o.Usages)
	if err != nil {
		return err
	}

	if err := tokenutil.CreateNewToken(client, bootstrapToken); err != nil {
		return err
	}

	tokenStr := bootstrapToken.Token.ID + "." + bootstrapToken.Token.Secret

	// if --print-register-command was specified, print a machine-readable full `karmadactl register` command
	// otherwise, just print the token
	if o.PrintRegisterCommand {
		joinCommand, err := tokenutil.GenerateRegisterCommand(o.KubeConfig, o.parentCommand, tokenStr)
		if err != nil {
			return fmt.Errorf("failed to get register command, err: %w", err)
		}
		fmt.Fprintln(out, joinCommand)
	} else {
		fmt.Fprintln(out, tokenStr)
	}

	return nil
}

// runListTokens lists details on all existing bootstrap tokens on the server.
func (o *CommandTokenOptions) runListTokens(client kubeclient.Interface, out io.Writer, errW io.Writer) error {
	// First, build our selector for bootstrap tokens only
	klog.V(1).Infoln("[token] preparing selector for bootstrap token")
	tokenSelector := fields.SelectorFromSet(
		map[string]string{
			"type": string(bootstrapapi.SecretTypeBootstrapToken),
		},
	)
	listOptions := metav1.ListOptions{
		FieldSelector: tokenSelector.String(),
	}

	klog.V(1).Info("[token] retrieving list of bootstrap tokens")
	secrets, err := client.CoreV1().Secrets(metav1.NamespaceSystem).List(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("failed to list bootstrap tokens, err: %w", err)
	}

	printFlags := get.NewGetPrintFlags()

	printFlags.SetKind(schema.GroupKind{Group: "output.karmada.io", Kind: "BootstrapToken"})

	printer, err := printFlags.ToPrinter()
	if err != nil {
		return err
	}

	// only print token with table format
	printer = &get.TablePrinter{Delegate: printer}

	tokenTable := &metav1.Table{}
	setColumnDefinition(tokenTable)

	for idx := range secrets.Items {
		// Get the BootstrapToken struct representation from the Secret object
		token, err := tokenutil.GetBootstrapTokenFromSecret(&secrets.Items[idx])
		if err != nil {
			fmt.Fprintf(errW, "%v", err)
			continue
		}

		outputToken := tokenutil.BootstrapToken{
			Token:       &tokenutil.Token{ID: token.Token.ID, Secret: token.Token.Secret},
			Description: token.Description,
			TTL:         token.TTL,
			Expires:     token.Expires,
			Usages:      token.Usages,
			Groups:      token.Groups,
		}

		tokenTable.Rows = append(tokenTable.Rows, constructTokenTableRow(outputToken))
	}

	if err := printer.PrintObj(tokenTable, out); err != nil {
		return fmt.Errorf("unable to print tokens, err: %w", err)
	}

	return nil
}

// runDeleteTokens removes a bootstrap tokens from the server.
func (o *CommandTokenOptions) runDeleteTokens(out io.Writer, client kubeclient.Interface, tokenIDsOrTokens []string) error {
	for _, tokenIDOrToken := range tokenIDsOrTokens {
		// Assume this is a token id and try to parse it
		tokenID := tokenIDOrToken
		klog.V(1).Info("[token] parsing token")
		if !bootstraputil.IsValidBootstrapTokenID(tokenIDOrToken) {
			// Okay, the full token with both id and secret was probably passed. Parse it and extract the ID only
			bts, err := tokenutil.NewToken(tokenIDOrToken)
			if err != nil {
				return fmt.Errorf("given token didn't match pattern %q or %q",
					bootstrapapi.BootstrapTokenIDPattern, bootstrapapi.BootstrapTokenIDPattern)
			}
			tokenID = bts.ID
		}

		tokenSecretName := bootstraputil.BootstrapTokenSecretName(tokenID)
		klog.V(1).Infof("[token] deleting token %q", tokenID)
		if err := client.CoreV1().Secrets(metav1.NamespaceSystem).Delete(context.TODO(), tokenSecretName, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete bootstrap token %q, err: %w", tokenID, err)
		}
		fmt.Fprintf(out, "bootstrap token %q deleted\n", tokenID)
	}
	return nil
}

// getClientSet get clientset of karmada control plane
func (o *CommandTokenOptions) getClientSet(karmadaConfig KarmadaConfig) (kubeclient.Interface, error) {
	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(o.KarmadaContext, o.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			o.KarmadaContext, o.KubeConfig, err)
	}
	return kubeclient.NewForConfigOrDie(controlPlaneRestConfig), nil
}

// constructTokenTableRow construct token table row
func constructTokenTableRow(token tokenutil.BootstrapToken) metav1.TableRow {
	var row metav1.TableRow

	tokenString := token.Token.ID + "." + token.Token.Secret

	ttl := "<forever>"
	expires := "<never>"
	if token.Expires != nil {
		ttl = duration.ShortHumanDuration(time.Until(token.Expires.Time))
		expires = token.Expires.Format(time.RFC3339)
	}

	usages := strings.Join(token.Usages, ",")
	if len(usages) == 0 {
		usages = "<none>"
	}

	description := token.Description
	if len(description) == 0 {
		description = "<none>"
	}

	groups := strings.Join(token.Groups, ",")
	if len(groups) == 0 {
		groups = "<none>"
	}

	row.Cells = append(row.Cells, tokenString, ttl, expires, usages, description, groups)

	return row
}

// setColumnDefinition set print ColumnDefinition
func setColumnDefinition(table *metav1.Table) {
	tokenColumns := []metav1.TableColumnDefinition{
		{Name: "TOKEN", Type: "string", Format: "", Priority: 0},
		{Name: "TTL", Type: "string", Format: "", Priority: 0},
		{Name: "EXPIRES", Type: "string", Format: "", Priority: 0},
		{Name: "USAGES", Type: "string", Format: "", Priority: 0},
		{Name: "DESCRIPTION", Type: "string", Format: "", Priority: 0},
		{Name: "EXTRA GROUPS", Type: "string", Format: "", Priority: 0},
	}
	table.ColumnDefinitions = tokenColumns
}
