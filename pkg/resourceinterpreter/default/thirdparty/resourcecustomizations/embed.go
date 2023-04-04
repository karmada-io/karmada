package resourcecustomizations

import (
	"embed"
)

// Embedded contains embedded resource customization
//
//go:embed *
var Embedded embed.FS
