// Package version provides build-time version information.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables injected via ldflags.
var (
	// Version is the semantic version (e.g., "1.2.3")
	Version = "dev"
	// Commit is the git commit SHA
	Commit = "none"
	// Date is the build date
	Date = "unknown"
)

// Info returns formatted version information.
func Info() string {
	return fmt.Sprintf("sacha %s (%s) built %s with %s",
		Version, Commit, Date, runtime.Version())
}

// Short returns just the version string.
func Short() string {
	return Version
}
