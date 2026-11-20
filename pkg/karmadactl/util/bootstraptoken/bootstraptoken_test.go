/*
Copyright 2026 The Karmada Authors.

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

package bootstraptoken

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// With a single step TryRunCommand never sleeps, so this exercises the exported
// function directly without waiting for the real backoff.
func TestTryRunCommand(t *testing.T) {
	wantErr := errors.New("secrets \"bootstrap-token-abcdef\" is forbidden")

	t.Run("returns the error of the failed attempt", func(t *testing.T) {
		calls := 0
		err := TryRunCommand(func() error {
			calls++
			return wantErr
		}, 1)
		if !errors.Is(err, wantErr) {
			t.Fatalf("TryRunCommand() = %v, want %v", err, wantErr)
		}
		if calls != 1 {
			t.Errorf("function called %d times, want 1", calls)
		}
	})

	t.Run("returns nil when the function succeeds", func(t *testing.T) {
		if err := TryRunCommand(func() error { return nil }, 1); err != nil {
			t.Fatalf("TryRunCommand() = %v, want nil", err)
		}
	})
}

func TestTryRunCommandWithBackoff(t *testing.T) {
	const steps = 3
	backoff := wait.Backoff{Duration: time.Millisecond, Factor: 2, Steps: steps}

	tests := []struct {
		name      string
		succeedAt int // 1-based attempt that succeeds, 0 means never
		wantCalls int
		wantErr   error
	}{
		{
			name:      "succeeds on the first attempt",
			succeedAt: 1,
			wantCalls: 1,
		},
		{
			name:      "succeeds after transient failures",
			succeedAt: 2,
			wantCalls: 2,
		},
		{
			name:      "returns the last error when every attempt fails",
			succeedAt: 0,
			wantCalls: steps,
			wantErr:   fmt.Errorf("attempt %d failed", steps),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := tryRunCommand(func() error {
				calls++
				if calls == tt.succeedAt {
					return nil
				}
				return fmt.Errorf("attempt %d failed", calls)
			}, backoff)

			if calls != tt.wantCalls {
				t.Errorf("function called %d times, want %d", calls, tt.wantCalls)
			}
			switch {
			case tt.wantErr == nil && err != nil:
				t.Errorf("tryRunCommand() = %v, want nil", err)
			case tt.wantErr != nil && (err == nil || err.Error() != tt.wantErr.Error()):
				t.Errorf("tryRunCommand() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestTryRunCommandWithBackoffNoSteps(t *testing.T) {
	calls := 0
	err := tryRunCommand(func() error {
		calls++
		return errors.New("must not be called")
	}, wait.Backoff{Duration: time.Millisecond, Factor: 2, Steps: 0})

	if calls != 0 {
		t.Errorf("function called %d times, want 0", calls)
	}
	if !wait.Interrupted(err) {
		t.Errorf("tryRunCommand() = %v, want the generic timeout error", err)
	}
}
