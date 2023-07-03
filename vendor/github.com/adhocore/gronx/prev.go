package gronx

import (
	"fmt"
	"time"
)

// PrevTick gives previous run time before now
func PrevTick(expr string, inclRefTime bool) (time.Time, error) {
	return PrevTickBefore(expr, time.Now(), inclRefTime)
}

// PrevTickBefore gives previous run time before given reference time
func PrevTickBefore(expr string, start time.Time, inclRefTime bool) (time.Time, error) {
	gron, prev := New(), start.Truncate(time.Second)
	due, err := gron.IsDue(expr, start)
	if err != nil || (due && inclRefTime) {
		return prev, err
	}

	segments, _ := Segments(expr)
	if len(segments) > 6 && isUnreachableYear(segments[6], prev, inclRefTime, true) {
		return prev, fmt.Errorf("unreachable year segment: %s", segments[6])
	}

	prev, err = loop(gron, segments, prev, inclRefTime, true)
	// Ignore superfluous err
	if err != nil && gron.isDue(expr, prev) {
		err = nil
	}
	return prev, err
}
