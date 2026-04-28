package gronx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Checker is interface for cron segment due check.
type Checker interface {
	GetRef() time.Time
	SetRef(ref time.Time)
	CheckDue(segment string, pos int) (bool, error)
}

// SegmentChecker is factory implementation of Checker.
type SegmentChecker struct {
	ref time.Time
}

// GetRef returns the current reference time
func (c *SegmentChecker) GetRef() time.Time {
	return c.ref
}

// SetRef sets the reference time for which to check if a cron expression is due.
func (c *SegmentChecker) SetRef(ref time.Time) {
	c.ref = ref
}

// CheckDue checks if the cron segment at given position is due.
// It returns bool or error if any.
func (c *SegmentChecker) CheckDue(segment string, pos int) (due bool, err error) {
	ref, last := c.GetRef(), -1
	val, loc := valueByPos(ref, pos), ref.Location()
	isMonth, isWeekDay := pos == 3, pos == 5

	for _, offset := range strings.Split(segment, ",") {
		mod := (isMonth || isWeekDay) && strings.ContainsAny(offset, "LW#")
		if due, err = c.isOffsetDue(offset, val, pos); due || (!mod && err != nil) {
			return
		}
		if !mod {
			continue
		}
		if last == -1 {
			last = time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, loc).AddDate(0, 1, 0).Add(-time.Nanosecond).Day()
		}
		if isMonth {
			due, err = isValidMonthDay(offset, last, ref)
		} else if isWeekDay {
			due, err = isValidWeekDay(offset, last, ref)
		}
		if due || err != nil {
			return due, err
		}
	}

	return false, nil
}

func (c *SegmentChecker) isOffsetDue(offset string, val, pos int) (bool, error) {
	if offset == "*" || offset == "?" {
		return true, nil
	}

	bounds, isWeekDay := boundsByPos(pos), pos == 5
	if strings.Contains(offset, "/") {
		return inStep(val, offset, bounds)
	}
	if strings.Contains(offset, "-") {
		if isWeekDay {
			offset = strings.Replace(offset, "7-", "0-", 1)
		}
		return inRange(val, offset, bounds)
	}

	if !isWeekDay && (val == 0 || offset == "0") {
		return offset == "0" && val == 0, nil
	}

	nval, err := strconv.Atoi(offset)
	if err != nil {
		return false, err
	}
	if nval < bounds[0] || nval > bounds[1] {
		return false, fmt.Errorf("segment#%d: '%s' out of bounds(%d, %d)", pos, offset, bounds[0], bounds[1])
	}

	return nval == val || (isWeekDay && nval == 7 && val == 0), nil
}

func valueByPos(ref time.Time, pos int) (val int) {
	switch pos {
	case 0:
		val = ref.Second()
	case 1:
		val = ref.Minute()
	case 2:
		val = ref.Hour()
	case 3:
		val = ref.Day()
	case 4:
		val = int(ref.Month())
	case 5:
		val = int(ref.Weekday())
	case 6:
		val = ref.Year()
	}
	return
}

func boundsByPos(pos int) (bounds []int) {
	bounds = []int{0, 0}
	switch pos {
	case 0, 1:
		bounds = []int{0, 59}
	case 2:
		bounds = []int{0, 23}
	case 3:
		bounds = []int{1, 31}
	case 4:
		bounds = []int{1, 12}
	case 5:
		bounds = []int{0, 7}
	case 6:
		bounds = []int{1, 9999}
	}
	return
}
