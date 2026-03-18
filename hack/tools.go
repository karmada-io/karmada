//go:build tools
// +build tools

package tools

import (
	_ "github.com/vektra/mockery/v3"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
