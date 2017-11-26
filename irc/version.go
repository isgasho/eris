package irc

import (
	"fmt"
)

var (
	//PackageName package name
	Package = "eris"

	// Version release version
	Version = "1.6.1"

	// Build will be overwritten automatically by the build system
	Build = "dev"

	// GitCommit will be overwritten automatically by the build system
	GitCommit = "HEAD"
)

// FullVersion display the full version and build
func FullVersion() string {
	return fmt.Sprintf("%s-%s-%s@%s", Package, Version, Build, GitCommit)
}
