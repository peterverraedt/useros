package useros

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

var ErrTypeAssertion = errors.New("type assertion")

func (u *user) CheckOwnership(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	return u.checkOwnership(stat)
}

func (u *user) LcheckOwnership(name string) error {
	stat, err := os.Lstat(name)
	if err != nil {
		return err
	}

	return u.checkOwnership(stat)
}

type Permission uint32

const (
	Read    Permission = 4
	Write   Permission = 2
	Execute Permission = 1
)

func (p Permission) User() uint32 {
	return uint32(p) * 0100
}

func (p Permission) Group() uint32 {
	return uint32(p) * 010
}

func (p Permission) Other() uint32 {
	return uint32(p)
}

func (u *user) CheckPermission(name string, permission Permission) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, permission)
}

func (u *user) LcheckPermission(name string, permission Permission) error {
	stat, err := os.Lstat(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, permission)
}

func (u *user) canTraverseParents(name string) error {
	dirs, err := Directories(name)
	if err != nil {
		return err
	}

	checked := make(map[string]struct{})

	for _, dir := range dirs {
		if _, ok := checked[dir]; ok {
			continue
		}

		stat, err := os.Stat(dir)
		if err != nil {
			return err
		}

		if !stat.IsDir() {
			return syscall.ENOTDIR
		}

		err = u.checkPermission(stat, Execute)
		if err != nil {
			return err
		}

		checked[dir] = struct{}{}
	}

	return nil
}

func (u *user) checkOwnership(stat fs.FileInfo) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	if stat_t.Uid != uint32(u.UID) {
		return os.ErrPermission
	}

	return nil
}

// Check permissions
func (u *user) checkPermission(stat os.FileInfo, perms ...Permission) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	for _, perm := range perms {
		var access bool

		switch {
		case stat_t.Uid == uint32(u.UID):
			access = stat_t.Mode&perm.User() > 0
		case stat_t.Gid == uint32(u.GID):
			access = stat_t.Mode&perm.Group() > 0
		default:
			access = stat_t.Mode&perm.Other() > 0

			for _, g := range u.Groups {
				if stat_t.Gid == uint32(g) {
					access = stat_t.Mode&perm.Group() > 0
				}
			}
		}

		if !access {
			return os.ErrPermission
		}
	}

	return nil
}

func (u *user) chownNewFile(f *os.File, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return f.Chown(u.UID, u.GID)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return f.Chown(u.UID, int(stat_t.Gid))
}

func (u *user) chownNewFolderOrSymlink(name string, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return os.Lchown(name, u.UID, u.GID)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return os.Lchown(name, u.UID, int(stat_t.Gid))
}
