package metrics

import "time"

const (
	// ResultSuccess represents the result of success.
	ResultSuccess = "success"
	// ResultError represents the result of error.
	ResultError = "error"
	// ResultUnknown represents the result of unknown.
	ResultUnknown = "unknown"
)

// GetResultByError returns results according to the error
func GetResultByError(err error) string {
	if err != nil {
		return ResultError
	}
	return ResultSuccess
}

// DurationInMicroseconds gets the time in microseconds.
func DurationInMicroseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / float64(time.Microsecond.Nanoseconds())
}

// DurationInMilliseconds gets the time in milliseconds.
func DurationInMilliseconds(start time.Time) float64 {
	return float64(time.Since(start).Nanoseconds()) / float64(time.Millisecond.Nanoseconds())
}

// DurationInSeconds gets the time in seconds.
func DurationInSeconds(start time.Time) float64 {
	return time.Since(start).Seconds()
}
