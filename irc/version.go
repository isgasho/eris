package irc

import (
	"fmt"
)

var (
	// Package package name
	Package = "eris"

	// Version release version
	Version = "1.6.4"

	// Commit will be overwritten automatically by the build system
	Commit = "HEAD"
)

// FullVersion display the full version and build
func FullVersion() string {
	return fmt.Sprintf("%s-%s@%s", Package, Version, Commit)
}
