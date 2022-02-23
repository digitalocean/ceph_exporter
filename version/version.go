package version

import (
	"errors"
	"strconv"
	"strings"
)

// Version contains all the Ceph version details
type Version struct {
	Major    int
	Minor    int
	Patch    int
	Revision int
	Commit   string
}

var (
	// ErrInvalidVersion indicates that the given version string was invalid
	ErrInvalidVersion = errors.New("invalid version")

	// Nautilus is the *Version at which Ceph nautilus was released
	Nautilus = &Version{Major: 14, Minor: 2, Patch: 0, Revision: 0, Commit: ""}

	// Octopus is the *Version at which Ceph octopus was released
	Octopus = &Version{Major: 15, Minor: 2, Patch: 0, Revision: 0, Commit: ""}

	// Pacific is the *Version at which Ceph pacific was released
	Pacific = &Version{Major: 16, Minor: 2, Patch: 0, Revision: 0, Commit: ""}
)

// IsAtLeast returns true if the version is at least as new as the given constraint
// the commit is not considered
func (version *Version) IsAtLeast(constraint *Version) bool {
	if version.Major > constraint.Major {
		return true
	} else if version.Major < constraint.Major {
		return false
	}

	if version.Minor > constraint.Minor {
		return true
	} else if version.Minor < constraint.Minor {
		return false
	}

	if version.Patch > constraint.Patch {
		return true
	} else if version.Patch < constraint.Patch {
		return false
	}

	if version.Revision > constraint.Revision {
		return true
	} else if version.Revision < constraint.Revision {
		return false
	}

	// the versions must be the same
	return true
}

// ParseCephVersion parses the given ceph version string to a *Version or error
func ParseCephVersion(cephVersion string) (*Version, error) {
	splitVersion := strings.Split(cephVersion, " ")
	if len(splitVersion) < 3 {
		return nil, ErrInvalidVersion
	}

	someVersions := strings.Split(splitVersion[2], ".")
	if len(someVersions) != 3 {
		return nil, ErrInvalidVersion
	}

	otherVersions := strings.Split(someVersions[2], "-")
	if len(otherVersions) != 3 {
		return nil, ErrInvalidVersion
	}

	major, err := strconv.Atoi(someVersions[0])
	if err != nil {
		return nil, err
	}

	minor, err := strconv.Atoi(someVersions[1])
	if err != nil {
		return nil, err
	}

	patch, err := strconv.Atoi(otherVersions[0])
	if err != nil {
		return nil, err
	}

	revision, err := strconv.Atoi(otherVersions[1])
	if err != nil {
		return nil, err
	}

	commit := otherVersions[2]

	return &Version{
		Major:    major,
		Minor:    minor,
		Patch:    patch,
		Revision: revision,
		Commit:   commit,
	}, nil
}
