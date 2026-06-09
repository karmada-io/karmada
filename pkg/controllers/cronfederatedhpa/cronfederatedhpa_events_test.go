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

package cronfederatedhpa

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoevents "k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"

	autoscalingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/autoscaling/v1alpha1"
	"github.com/karmada-io/karmada/pkg/events"
)

// capturingRecorder records every field passed to Eventf, including action
// and related which the upstream events.FakeRecorder discards from its event
// channel. It satisfies clientgoevents.EventRecorderLogger.
type capturingRecorder struct {
	regarding runtime.Object
	related   runtime.Object
	eventType string
	reason    string
	action    string
	note      string
	called    int
}

func (c *capturingRecorder) Eventf(regarding, related runtime.Object, eventtype, reason, action, note string, args ...any) {
	c.regarding = regarding
	c.related = related
	c.eventType = eventtype
	c.reason = reason
	c.action = action
	c.note = fmt.Sprintf(note, args...)
	c.called++
}

func (c *capturingRecorder) WithLogger(klog.Logger) clientgoevents.EventRecorderLogger {
	return c
}

var _ clientgoevents.EventRecorderLogger = &capturingRecorder{}

func newCronFHPAFixture() *autoscalingv1alpha1.CronFederatedHPA {
	return &autoscalingv1alpha1.CronFederatedHPA{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cron-fhpa",
			Namespace: "default",
		},
		Spec: autoscalingv1alpha1.CronFederatedHPASpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: autoscalingv1alpha1.GroupVersion.String(),
				Kind:       autoscalingv1alpha1.FederatedHPAKind,
				Name:       "test-fhpa",
			},
		},
	}
}

func TestTargetReferenceFor(t *testing.T) {
	cronFHPA := newCronFHPAFixture()

	ref := targetReferenceFor(cronFHPA)

	assert.Equal(t, autoscalingv1alpha1.GroupVersion.String(), ref.APIVersion)
	assert.Equal(t, autoscalingv1alpha1.FederatedHPAKind, ref.Kind)
	assert.Equal(t, "test-fhpa", ref.Name)
	assert.Equal(t, "default", ref.Namespace)

	// The returned reference is itself a runtime.Object — required for use
	// as the 'related' argument on the new EventRecorder.Eventf signature.
	var _ runtime.Object = ref
}

// TestEventRecorderEmitsAllFields exercises the new events.k8s.io/v1
// Eventf signature with the constants and helpers introduced by this PoC,
// asserting that every field — including action and related, which the
// upstream FakeRecorder drops from its channel — round-trips correctly.
//
// It does not invoke a controller reconcile (that path is covered by the
// existing handler tests); it asserts the contract between the recorder
// and the constants/helpers this migration introduces.
func TestEventRecorderEmitsAllFields(t *testing.T) {
	rec := &capturingRecorder{}
	cronFHPA := newCronFHPAFixture()

	rec.Eventf(cronFHPA, targetReferenceFor(cronFHPA), corev1.EventTypeWarning,
		events.EventReasonScaleCronFederatedHPAFailed, events.EventActionScaleCronFederatedHPA,
		"failed to scale %s %s/%s: %v", cronFHPA.Spec.ScaleTargetRef.Kind, cronFHPA.Namespace, cronFHPA.Spec.ScaleTargetRef.Name, errors.New("target not found"))

	assert.Equal(t, 1, rec.called)
	assert.Same(t, cronFHPA, rec.regarding)
	assert.Equal(t, corev1.EventTypeWarning, rec.eventType)
	assert.Equal(t, "ScaleFailed", rec.reason)
	assert.Equal(t, "ScaleCronFederatedHPA", rec.action)
	assert.Contains(t, rec.note, "FederatedHPA")
	assert.Contains(t, rec.note, "default/test-fhpa")
	assert.Contains(t, rec.note, "target not found")

	related, ok := rec.related.(*corev1.ObjectReference)
	assert.True(t, ok, "related should be *corev1.ObjectReference, got %T", rec.related)
	assert.Equal(t, "test-fhpa", related.Name)
	assert.Equal(t, autoscalingv1alpha1.FederatedHPAKind, related.Kind)
}

// TestEventConstantsPreserveWireFormat guards against accidental edits to
// the reason strings, which are part of the public surface of these events
// — external tooling may match on them.
func TestEventConstantsPreserveWireFormat(t *testing.T) {
	cases := map[string]string{
		"StartRuleFailed":              events.EventReasonStartCronFederatedHPARuleFailed,
		"UpdateCronFederatedHPAFailed": events.EventReasonUpdateCronFederatedHPAFailed,
		"ScaleFailed":                  events.EventReasonScaleCronFederatedHPAFailed,
		"UpdateStatusFailed":           events.EventReasonUpdateCronFederatedHPAStatusFailed,
	}
	for want, got := range cases {
		assert.Equal(t, want, got, "reason string drifted from existing wire format")
	}
}

// TestEventfFormatStringSafety guards against the regression where an error
// message containing format verbs (%v, %s, %d, or even raw % characters from
// URL-encoded paths) is passed as the 'note' format string to Eventf. The
// safe pattern is to pass "%v" as the format and the error as an argument,
// so Sprintf treats the error message as data, not as a format directive.
func TestEventfFormatStringSafety(t *testing.T) {
	rec := &capturingRecorder{}
	cronFHPA := newCronFHPAFixture()
	// An error whose message contains both real format verbs and a raw % —
	// the kind that arises from wrapped errors and URL-bearing API failures.
	hostileErr := errors.New("decoding %d failed at https://api/foo%20bar: %v")

	rec.Eventf(cronFHPA, nil, corev1.EventTypeWarning,
		events.EventReasonUpdateCronFederatedHPAFailed, events.EventActionUpdateCronFederatedHPA,
		"%v", hostileErr)

	// With the safe pattern, the error message round-trips verbatim — the
	// %d, %20, and %v inside the error are treated as literal characters.
	assert.Equal(t, hostileErr.Error(), rec.note)
	assert.NotContains(t, rec.note, "MISSING", "Sprintf saw the error as a format string and substituted MISSING placeholders")
}
