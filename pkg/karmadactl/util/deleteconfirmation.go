/*
Copyright 2023 The Karmada Authors.

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

import (
	"fmt"
	"strings"
)

// DeleteConfirmation prompts the user for a yes/no confirmation.
// It returns true if the user confirms with "y" or "yes", false on "n" or "no",
// and an error if stdin cannot be read (e.g. non-interactive environment).
func DeleteConfirmation() (bool, error) {
	for {
		fmt.Print("Please type (y)es or (n)o and then press enter:")
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil {
			return false, fmt.Errorf("failed to read user confirmation: %w", err)
		}

		switch strings.ToLower(response) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("invalid input, please type (y)es or (n)o")
		}
	}
}
