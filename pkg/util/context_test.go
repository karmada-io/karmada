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

package util

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContextForChannel(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() (chan struct{}, func(context.Context, context.CancelFunc))
		expectedDone bool
		timeout      time.Duration
	}{
		{
			name: "context is cancelled when cancel function is called",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, cancel context.CancelFunc) {
					cancel()
				}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
		{
			name: "context is cancelled when parent channel is closed",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, _ context.CancelFunc) {
					close(ch)
				}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
		{
			name: "context remains open when neither cancelled nor parent channel closed",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, _ context.CancelFunc) {
					// Do nothing - context should remain open
				}
			},
			expectedDone: false,
			timeout:      100 * time.Millisecond,
		},
		{
			name: "concurrent operations - cancel first, then close parent",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, cancel context.CancelFunc) {
					go cancel()
					go func() {
						time.Sleep(50 * time.Millisecond)
						close(ch)
					}()
				}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
		{
			name: "concurrent operations - close parent first, then cancel",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, cancel context.CancelFunc) {
					go close(ch)
					go func() {
						time.Sleep(50 * time.Millisecond)
						cancel()
					}()
				}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
		{
			name: "multiple cancel calls should not panic",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				return ch, func(_ context.Context, cancel context.CancelFunc) {
					cancel()
					cancel() // Second call should not panic
					cancel() // Third call should not panic
				}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
		{
			name: "parent channel already closed",
			setup: func() (chan struct{}, func(context.Context, context.CancelFunc)) {
				ch := make(chan struct{})
				close(ch)
				return ch, func(_ context.Context, _ context.CancelFunc) {}
			},
			expectedDone: true,
			timeout:      time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parentCh, operation := tt.setup()
			ctx, cancel := ContextForChannel(parentCh)
			defer cancel() // Always clean up

			// Run the test operation
			operation(ctx, cancel)

			// Check if context is done within timeout
			select {
			case <-ctx.Done():
				assert.True(t, tt.expectedDone, "context was cancelled but expected to remain open")
			case <-time.After(tt.timeout):
				assert.False(t, tt.expectedDone, "context remained open but expected to be cancelled")
			}
		})
	}
}
