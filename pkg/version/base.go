package version

// Base version information.
//
// This is the fallback data used when version information from git is not
// provided via go ldflags. It provides an approximation of the Karmada
// version for ad-hoc builds (e.g. `go build`) that cannot get the version
// information from git.
var (
	gitVersion   = "v0.0.0-master"
	gitCommit    = "unknown" // sha1 from git, output of $(git rev-parse HEAD)
	gitTreeState = "unknown" // state of git tree, either "clean" or "dirty"

	buildDate = "unknown" // build date in ISO8601 format, output of $(date -u +'%Y-%m-%dT%H:%M:%SZ')
)
