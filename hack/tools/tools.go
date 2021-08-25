// +build tools

package tools

import (
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "github.com/onsi/ginkgo/ginkgo"
	_ "golang.org/x/tools/cmd/goimports"
	_ "k8s.io/code-generator"
)
