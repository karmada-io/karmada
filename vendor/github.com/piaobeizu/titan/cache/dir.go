//go:build !windows
// +build !windows

package cache

import (
	"os"
	"path"
	"runtime"

	"golang.org/x/sys/unix"
)

// Dir returns the directory where deepai temporary files should be stored. The directory will be created if it does not exist.
func Dir() string {
	d := defaultdir()
	if EnsureDir(d) == nil && unix.Access(d, unix.W_OK) == nil {
		return d
	}
	return fallbackdir()
}

func defaultdir() string {
	switch runtime.GOOS {
	case "linux":
		return "/var/cache/titan"
	case "darwin":
		home, _ := os.UserHomeDir()
		return path.Join(home, "Library", "Caches", "deepai")
	default:
		return path.Join(os.TempDir(), "deepai")
	}
}

func fallbackdir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	fb := path.Join(home, ".deepai", "cache")
	if err := EnsureDir(fb); err != nil {
		return ""
	}
	return fb
}
