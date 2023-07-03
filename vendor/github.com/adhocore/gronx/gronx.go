package gronx

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var literals = strings.NewReplacer(
	"SUN", "0", "MON", "1", "TUE", "2", "WED", "3", "THU", "4", "FRI", "5", "SAT", "6",
	"JAN", "1", "FEB", "2", "MAR", "3", "APR", "4", "MAY", "5", "JUN", "6", "JUL", "7",
	"AUG", "8", "SEP", "9", "OCT", "10", "NOV", "11", "DEC", "12",
)

var expressions = map[string]string{
	"@yearly":    "0 0 1 1 *",
	"@annually":  "0 0 1 1 *",
	"@monthly":   "0 0 1 * *",
	"@weekly":    "0 0 * * 0",
	"@daily":     "0 0 * * *",
	"@hourly":    "0 * * * *",
	"@always":    "* * * * *",
	"@5minutes":  "*/5 * * * *",
	"@10minutes": "*/10 * * * *",
	"@15minutes": "*/15 * * * *",
	"@30minutes": "0,30 * * * *",

	"@everysecond": "* * * * * *",
}

// SpaceRe is regex for whitespace.
var SpaceRe = regexp.MustCompile(`\s+`)
var yearRe = regexp.MustCompile(`\d{4}`)

func normalize(expr string) []string {
	expr = strings.Trim(expr, " \t")
	if e, ok := expressions[strings.ToLower(expr)]; ok {
		expr = e
	}

	expr = SpaceRe.ReplaceAllString(expr, " ")
	expr = literals.Replace(strings.ToUpper(expr))

	return strings.Split(strings.ReplaceAll(expr, "  ", " "), " ")
}

// Gronx is the main program.
type Gronx struct {
	C Checker
}

// New initializes Gronx with factory defaults.
func New() Gronx {
	return Gronx{&SegmentChecker{}}
}

// IsDue checks if cron expression is due for given reference time (or now).
// It returns bool or error if any.
func (g *Gronx) IsDue(expr string, ref ...time.Time) (bool, error) {
	ref = append(ref, time.Now())
	g.C.SetRef(ref[0])

	segs, err := Segments(expr)
	if err != nil {
		return false, err
	}

	return g.SegmentsDue(segs)
}

func (g *Gronx) isDue(expr string, ref time.Time) bool {
	due, err := g.IsDue(expr, ref)
	return err == nil && due
}

// Segments splits expr into array array of cron parts.
// If expression contains 5 parts or 6th part is year like, it prepends a second.
// It returns array or error.
func Segments(expr string) ([]string, error) {
	segs := normalize(expr)
	slen := len(segs)
	if slen < 5 || slen > 7 {
		return []string{}, errors.New("expr should contain 5-7 segments separated by space")
	}

	// Prepend second if required
	prepend := slen == 5 || (slen == 6 && yearRe.MatchString(segs[5]))
	if prepend {
		segs = append([]string{"0"}, segs...)
	}

	return segs, nil
}

// SegmentsDue checks if all cron parts are due.
// It returns bool. You should use IsDue(expr) instead.
func (g *Gronx) SegmentsDue(segs []string) (bool, error) {
	for pos, seg := range segs {
		if seg == "*" || seg == "?" {
			continue
		}

		if due, err := g.C.CheckDue(seg, pos); !due {
			return due, err
		}
	}

	return true, nil
}

// IsValid checks if cron expression is valid.
// It returns bool.
func (g *Gronx) IsValid(expr string) bool {
	segs, err := Segments(expr)
	if err != nil {
		return false
	}

	g.C.SetRef(time.Now())
	for pos, seg := range segs {
		if _, err := g.C.CheckDue(seg, pos); err != nil {
			return false
		}
	}

	return true
}
