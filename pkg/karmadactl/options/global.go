package options

import "time"

// DefaultKarmadaClusterNamespace defines the default namespace where the member cluster secrets are stored.
const DefaultKarmadaClusterNamespace = "karmada-cluster"

// DefaultKarmadactlCommandDuration defines the default timeout for karmadactl execute
const DefaultKarmadactlCommandDuration = 60 * time.Second
