package useros

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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
	return uint32(p)
}

func (p Permission) Group() uint32 {
	return uint32(p) * 010
}

func (p Permission) Other() uint32 {
	return uint32(p) * 0100
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
	name, err := filepath.Abs(name)
	if err != nil {
		return err
	}

	if name == "/" || name == "" {
		return nil
	}

	parent := filepath.Dir(name)

	err = u.canTraverseParents(parent)
	if err != nil {
		return err
	}

	stat, err := u.Stat(parent)
	if err != nil {
		return err
	}

	// The parent should be a directory
	if !stat.IsDir() {
		return syscall.ENOTDIR
	}

	return u.checkPermission(stat, Execute)
}

func (u *user) checkOwnership(stat fs.FileInfo) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	if stat_t.Uid != uint32(u.Uid) {
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

outer:
	for _, perm := range perms {
		switch {
		case stat_t.Uid == uint32(u.Uid) && stat_t.Mode&perm.User() > 0:
			continue
		case stat_t.Gid == uint32(u.Gid) && stat_t.Mode&perm.Group() > 0:
			continue
		case stat_t.Mode&perm.Other() > 0:
			continue
		}

		if stat_t.Mode&perm.Group() > 0 {
			for _, g := range u.Groups {
				if stat_t.Gid == uint32(g) {
					continue outer
				}
			}
		}

		return os.ErrPermission
	}

	return nil
}

func (u *user) chownNewFile(f *os.File, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return f.Chown(u.Uid, u.Gid)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return f.Chown(u.Uid, int(stat_t.Gid))
}

func (u *user) chownNewFolderOrSymlink(name string, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return os.Lchown(name, u.Uid, u.Gid)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return os.Lchown(name, u.Uid, int(stat_t.Gid))
}
