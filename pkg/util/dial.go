package util

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
)

// Dial establishes the gRPC communication.
func Dial(path string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
	}
	cc, err := grpc.DialContext(ctx, path, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s error: %v", path, err)
	}

	return cc, nil
}
