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

package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	workv1alpha1 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha1"
)

// newWorkWithGenAndDeletion returns a Work with the specified generation and deletion timestamp presence.
func newWorkWithGenAndDeletion(generation int64, deleted bool) *workv1alpha1.Work {
    w := &workv1alpha1.Work{}
    w.SetGeneration(generation)
    if deleted {
        now := metav1.Now()
        w.SetDeletionTimestamp(&now)
    }
    return w
}

func TestBuildWorkEventPredicate_Update_GenerationChange(t *testing.T) {
    pred := buildWorkEventPredicate()

    oldObj := newWorkWithGenAndDeletion(1, false)
    newObj := newWorkWithGenAndDeletion(2, false)

    allow := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
    assert.True(t, allow)
}

func TestBuildWorkEventPredicate_Update_DeletionTimestamp_NilToNonNil(t *testing.T) {
    pred := buildWorkEventPredicate()

    oldObj := newWorkWithGenAndDeletion(1, false)
    newObj := newWorkWithGenAndDeletion(1, true)

    allow := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
    assert.True(t, allow)
}

func TestBuildWorkEventPredicate_Update_DeletionTimestamp_NonNilToNonNil(t *testing.T) {
    pred := buildWorkEventPredicate()

    oldObj := newWorkWithGenAndDeletion(1, true)
    newObj := newWorkWithGenAndDeletion(1, true)

    allow := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
    assert.False(t, allow)
}

func TestBuildWorkEventPredicate_Update_NoChange(t *testing.T) {
    pred := buildWorkEventPredicate()

    oldObj := newWorkWithGenAndDeletion(1, false)
    newObj := newWorkWithGenAndDeletion(1, false)

    allow := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
    assert.False(t, allow)
}

func TestBuildWorkEventPredicate_Delete_Ignored(t *testing.T) {
    pred := buildWorkEventPredicate()

    obj := newWorkWithGenAndDeletion(1, true)
    allow := pred.Delete(event.DeleteEvent{Object: obj})
    assert.False(t, allow)
}

func TestBuildWorkEventPredicate_Update_NilSafety(t *testing.T) {
    pred := buildWorkEventPredicate()

    obj := newWorkWithGenAndDeletion(1, false)

    assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: nil, ObjectNew: obj}))
    assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: obj, ObjectNew: nil}))
}


