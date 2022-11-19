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

// DurationInSeconds gets the time in seconds.
func DurationInSeconds(start time.Time) float64 {
	return time.Since(start).Seconds()
}
