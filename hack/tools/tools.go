//go:build tools
// +build tools

package tools

import (
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "github.com/vektra/mockery/v2"
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
)
