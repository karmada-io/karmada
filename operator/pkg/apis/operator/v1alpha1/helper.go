package v1alpha1

import (
	"fmt"
)

// Name returns the image name.
func (image *Image) Name() string {
	return fmt.Sprintf("%s:%s", image.ImageRepository, image.ImageTag)
}
