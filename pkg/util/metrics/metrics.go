/*
Copyright 2021 The Karmada Authors.

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
