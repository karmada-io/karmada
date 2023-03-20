package util

import (
	"fmt"
	"os"
	"strings"
)

// DeleteConfirmation delete karmada resource confirmation
func DeleteConfirmation() bool {
	fmt.Print("Please type (y)es or (n)o and then press enter:")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	default:
		return DeleteConfirmation()
	}
}
