/*
Copyright 2025 The Karmada Authors.

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

package queue

import (
	"time"
)

// QueuedBindingInfo is a Binding wrapper with additional information related to
// the binding's status in the scheduling queue, such as the timestamp when
// it's added to the queue.
type QueuedBindingInfo struct {
	NamespacedKey string

	// The priority of ResourceBinding.
	Priority int32

	// The time binding added to the scheduling queue.
	Timestamp time.Time

	// The time when the binding is added to the queue for the first time. The binding may be added
	// back to the queue multiple times before it's successfully scheduled.
	// It shouldn't be updated once initialized.
	InitialAttemptTimestamp *time.Time

	// Number of schedule attempts before successfully scheduled.
	// It's used to record the attempts metric and calculate the backoff time this Binding is obliged to get before retrying.
	Attempts int
}

// DeepCopy returns a deep copy of the QueuedBindingInfo object.
func (qbi *QueuedBindingInfo) DeepCopy() *QueuedBindingInfo {
	return &QueuedBindingInfo{
		NamespacedKey:           qbi.NamespacedKey,
		Priority:                qbi.Priority,
		Timestamp:               qbi.Timestamp,
		Attempts:                qbi.Attempts,
		InitialAttemptTimestamp: qbi.InitialAttemptTimestamp,
	}
}

// BindingKeyFunc is the key mapping function of QueuedBindingInfo.
func BindingKeyFunc(bindingInfo *QueuedBindingInfo) string {
	return bindingInfo.NamespacedKey
}

// Less is the function used by the activeQ heap algorithm to sort bindings.
// It sorts bindings based on their priority. When priorities are equal, it uses
// QueuedBindingInfo.timestamp.
func Less(bInfo1, bInfo2 *QueuedBindingInfo) bool {
	p1 := bInfo1.Priority
	p2 := bInfo2.Priority
	return (p1 > p2) || (p1 == p2 && bInfo1.Timestamp.Before(bInfo2.Timestamp))
}
