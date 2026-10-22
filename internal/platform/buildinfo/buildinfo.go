// Package buildinfo reports application version metadata from the compiled binary.
package buildinfo

import "runtime/debug"

// version is injected at release-build time via:
//
//	-ldflags "-X github.com/nguyentin05/spectest/internal/platform/buildinfo.version=vX.Y.Z"
var version string

// App returns the main module's version. A release build injects the tag via ldflags;
// otherwise it reads the compiled build metadata ("devel" for local `go run` / `go build`,
// or the module tag for `go install`).
func App() string {
	if version != "" {
		return version
	}
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi.Main.Version == "" || bi.Main.Version == "(devel)" {
		return "devel"
	}
	return bi.Main.Version
}
