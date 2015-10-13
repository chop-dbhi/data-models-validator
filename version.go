package validator

import "github.com/blang/semver"

// SemVer components.
const (
	progMajor        = 0
	progMinor        = 1
	progPatch        = 0
	progReleaseLevel = "beta"
	progReleaseNum   = 1
)

var (
	// Populated at build time. See the Makefile for details.
	// Note, in environments where the git information is not
	// available, these will not be populated.
	progBuild string

	// Full semantic version for the service.
	Version = semver.Version{
		Major: progMajor,
		Minor: progMinor,
		Patch: progPatch,
		Pre: []semver.PRVersion{{
			VersionStr: progReleaseLevel,
		}, {
			VersionNum: progReleaseNum,
			IsNum:      true,
		}},
	}
)

func init() {
	// Add the build if available.
	if progBuild != "" {
		Version.Build = []string{progBuild}
	}
}
