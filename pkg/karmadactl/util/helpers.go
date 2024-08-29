/*
Copyright 2024 The Karmada Authors.

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

package util

import "os"

// PathType represents the type of the path.
type PathType int

const (
	// File represents a file type.
	File PathType = iota
	// Directory represents a directory type.
	Directory
	// NonExistent represents a path that does not exist.
	NonExistent
)

// IsExist determines whether the specified path exists and returns its type.
// It returns PathType as Directory if the path is a directory,
// File if it is a file, and NonExistent if the path does not exist.
//
// Parameters:
//
//	path: The path to check.
//
// Returns:
//
//	PathType: The type of the path (File, Directory, NonExistent).
//	bool: Whether the path exists (true) or not (false).
func IsExist(path string) (PathType, bool) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NonExistent, false
		}
		// Error occurred while checking the path.
		return NonExistent, false
	}
	if info.IsDir() {
		return Directory, true
	}
	return File, true
}
