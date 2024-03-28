//   Copyright 2024 DigitalOcean
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package ceph

import (
	"errors"
	"fmt"
	"regexp"
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

	// Nautilus is the *Version at which Ceph Nautilus was released.
	Nautilus = &Version{Major: 14, Minor: 2, Patch: 0, Revision: 0, Commit: ""}

	// Octopus is the *Version at which Ceph Octopus was released.
	Octopus = &Version{Major: 15, Minor: 2, Patch: 0, Revision: 0, Commit: ""}

	// Pacific is the *Version at which Ceph Pacific was released.
	Pacific = &Version{Major: 16, Minor: 2, Patch: 0, Revision: 0, Commit: ""}

	// Quincy is the *Version at which Ceph Quincy was released.
	Quincy = &Version{Major: 17, Minor: 2, Patch: 0, Revision: 0, Commit: ""}
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

func (version *Version) String() string {
	str := fmt.Sprintf("%d.%d.%d", version.Major, version.Minor, version.Patch)
	if version.Revision != 0 || version.Commit != "" {
		str = fmt.Sprintf("%s-%d", str, version.Patch)
		if version.Commit != "" {
			str = fmt.Sprintf("%s-%s", str, version.Commit)
		}
	}

	return str
}

// ParseCephVersion parses the given ceph version string to a *Version or error
func ParseCephVersion(cephVersion string) (*Version, error) {
	// standardize ceph-ansible version format
	var re = regexp.MustCompile(`(\d+\.\d+\.\d+-\d+)\.(.*)`)
	cephVersion = re.ReplaceAllString(cephVersion, `$1-$2`)

	splitVersion := strings.Split(cephVersion, " ")
	if len(splitVersion) < 3 {
		return nil, ErrInvalidVersion
	}

	someVersions := strings.Split(splitVersion[2], ".")
	if len(someVersions) != 3 {
		return nil, ErrInvalidVersion
	}

	otherVersions := strings.Split(someVersions[2], "-")
	if len(otherVersions) == 1 {
		otherVersions = []string{otherVersions[0], "0", ""}
	} else if len(otherVersions) != 3 {
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
