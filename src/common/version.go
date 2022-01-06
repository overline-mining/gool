package common

import (
	"golang.org/x/mod/semver"
)

const version = "v0.0.1"

func GetVersion() string {
	return semver.Canonical(version)
}
