package file

import (
	"errors"
	"fmt"
	"os"
)

func Exists(name string) (bool, error) {
	info, err := os.Stat(name)
	if err == nil {
		if info.Mode().IsRegular() {
			return true, nil
		}
		return false, fmt.Errorf("%q is not a regular file", name)
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
