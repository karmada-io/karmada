package token

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/cluster-bootstrap/token/api"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

type mockIOWriter struct {
}

func (m *mockIOWriter) Write(p []byte) (n int, err error) {
	str := string(p)
	fmt.Println(str)
	return len(str), nil
}

func NewTestCommandTokenOptions() *CommandTokenOptions {
	return &CommandTokenOptions{
		TTL: &metav1.Duration{
			Duration: time.Second,
		},
		Description:   "foo",
		Groups:        []string{},
		Usages:        []string{},
		parentCommand: "foo",
	}
}

func TestNewCmdTokenCreate(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenCreate(f, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenCreate() return nil, but want config")
	}
}

func TestNewCmdTokenList(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenList(f, &mockIOWriter{}, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenList() return nil, but want config")
	}
}

func TestNewCmdTokenDelete(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenDelete(f, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenDelete() return nil, but want config")
	}
}

func TestCommandTokenOptions_runCreateToken(t *testing.T) {
	cmdOpt := NewTestCommandTokenOptions()
	client := kubernetesfake.NewSimpleClientset()
	if err := cmdOpt.runCreateToken(&mockIOWriter{}, client); err != nil {
		t.Errorf("runCreateToken() want nil, but get:%v", err)
	}
}

func TestCommandTokenOptions_runListTokens(t *testing.T) {
	tests := []struct {
		name        string
		tokenID     []string
		tokenSecret []string
		wantErr     bool
	}{
		{
			name: "token format error and parse error",
			//It needs special format, follow it will pass checking
			tokenID:     []string{"foo0id"},
			tokenSecret: []string{"---"},
			wantErr:     false,
		},
		{
			name: "token fmt ok and parse ok",
			//It needs special format, follow it will pass checking
			tokenID:     []string{"foo0id"},
			tokenSecret: []string{"foo0secret0foofo"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdOpt := NewTestCommandTokenOptions()
			client := kubernetesfake.NewSimpleClientset()
			ctx := context.Background()

			for i, id := range tt.tokenID {
				tokenSecret := corev1.Secret{
					Type: api.SecretTypeBootstrapToken,
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("bootstrap-token-%s", id),
					},
					Data: map[string][]byte{
						"token-id":     []byte(id),
						"token-secret": []byte(tt.tokenSecret[i]),
					},
				}

				if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).
					Create(ctx, &tokenSecret, metav1.CreateOptions{}); err != nil {
					t.Errorf("Create secret error:%v", err)
				}
			}

			if err := cmdOpt.runListTokens(client, &mockIOWriter{}, &mockIOWriter{}); (err != nil) != tt.wantErr {
				t.Errorf("runListTokens() want:%v, but get:%v", tt.wantErr, err)
			}
		})
	}
}

func TestCommandTokenOptions_runDeleteTokens(t *testing.T) {
	tests := []struct {
		name         string
		tokenID      string
		tokenInCache []string
		wantErr      bool
	}{
		{
			name:         "invalid format token id",
			tokenID:      "---",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{
			name:         "invalid format token id but match BootstrapTokenPattern",
			tokenID:      "foo0id.foo0secret0foofo",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{
			name:         "valid format but not exist in client",
			tokenID:      "foo0id",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{
			name:         "valid format and exist in client",
			tokenID:      "foo0id",
			tokenInCache: []string{"foo0id"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdOpt := NewTestCommandTokenOptions()
			client := kubernetesfake.NewSimpleClientset()
			ctx := context.Background()

			for _, tokenName := range tt.tokenInCache {
				tokenSecret := corev1.Secret{
					Type: api.SecretTypeBootstrapToken,
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("bootstrap-token-%s", tokenName),
					},
				}

				if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).
					Create(ctx, &tokenSecret, metav1.CreateOptions{}); err != nil {
					t.Errorf("Create secret error:%v", err)
				}
			}
			tokenList := []string{tt.tokenID}
			if err := cmdOpt.runDeleteTokens(&mockIOWriter{}, client, tokenList); (err != nil) != tt.wantErr {
				t.Errorf("runDeleteTokens() want:%v, but get:%v", tt.wantErr, err)
			}
		})
	}
}

func TestNewCmdToken(t *testing.T) {
	stream := genericclioptions.IOStreams{}
	f := util.NewFactory(options.DefaultConfigFlags)
	cmdToken := NewCmdToken(f, "karmadactl", stream)
	if cmdToken == nil {
		t.Errorf("NewCmdToken() want return not nil, but return nil")
	}
}
