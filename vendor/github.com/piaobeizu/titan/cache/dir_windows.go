// +build windows

package cache

import (
	"os"
	"path"

	"golang.org/x/sys/windows"
)

// Dir returns the directory where k0sctl temporary files should be stored
func Dir() string {
	appdata, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return path.Join(os.TempDir(), "k0sctl")
	}
	return path.Join(appdata, "k0sctl")
}
