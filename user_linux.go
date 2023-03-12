//go:build linux
// +build linux

package useros

import (
	"syscall"
)

func (u User) os() OS {
	// We should run as root, otherwise return the default os implementation
	if syscall.Geteuid() > 0 {
		return &def{}
	}

	// Assign default values if necessary
	if u.UID < 0 {
		u.UID = syscall.Geteuid()
	}

	if u.GID < 0 {
		u.GID = syscall.Getegid()
	}

	groups, _ := syscall.Getgroups() //nolint:errcheck

	if len(u.Groups) == 0 {
		u.Groups = groups
	}

	// Check whether we are impersonating a user
	if u.UID == syscall.Geteuid() && u.GID == syscall.Getegid() && equal(u.Groups, groups) {
		return &def{}
	}

	return &user{u}
}
