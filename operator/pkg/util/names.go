package util

import "fmt"

func ComponentName(component, name string) string {
	return fmt.Sprintf("%s-%s", name, component)
}

func ComponentImageName(repository, component, version string) string {
	return fmt.Sprintf("%s/%s:%s", repository, component, version)
}
