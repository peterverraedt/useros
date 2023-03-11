package useros

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/joshlf/go-acl"
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

func (p Permission) ACL() os.FileMode {
	return os.FileMode(p)
}

func (u *user) CheckPermission(name string, permission Permission) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	a, err := acl.Get(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, a, permission)
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

		acl, err := acl.Get(dir)
		if err != nil {
			return err
		}

		err = u.checkPermission(stat, acl, Execute)
		if err != nil {
			return err
		}

		checked[dir] = struct{}{}
	}

	return nil
}

func (u *user) canWriteInDirOf(name string) (fs.FileInfo, error) {
	dirname := filepath.Dir(name)

	// Can execute all parent directories?
	if err := u.canTraverseParents(dirname); err != nil {
		return nil, err
	}

	// Has write permissions
	stat, err := os.Stat(dirname)
	if err != nil {
		return nil, err
	}

	a, err := acl.Get(dirname)
	if err != nil {
		return nil, err
	}

	// Can create file?
	return stat, u.checkPermission(stat, a, Write, Execute)
}

func (u *user) checkOwnership(stat fs.FileInfo) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	if stat_t.Uid != uint32(u.UID) && u.UID > 0 {
		return os.ErrPermission
	}

	return nil
}

// Check permissions
func (u *user) checkPermission(stat os.FileInfo, a acl.ACL, perms ...Permission) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	groups := append([]int{u.GID}, u.Groups...)

	for _, perm := range perms {
		var access bool
		var aclmode os.FileMode

		switch {
		case stat_t.Uid == uint32(u.UID):
			access = stat_t.Mode&perm.User() > 0
		case find(a, acl.TagUser, u.UID, &aclmode):
			access = aclmode&aclmask(a)&perm.ACL() > 0
		default:
			foundGroup := false

			mask := aclmask(a)

			for _, g := range groups {
				if stat_t.Gid == uint32(g) {
					access = access || stat_t.Mode&(uint32(mask)*010)&perm.Group() > 0
					foundGroup = true
				}

				if find(a, acl.TagGroup, u.GID, &aclmode) {
					access = access || aclmode&mask&perm.ACL() > 0
					foundGroup = true
				}
			}

			if !foundGroup {
				access = stat_t.Mode&perm.Other() > 0
			}
		}

		if !access {
			return os.ErrPermission
		}
	}

	return nil
}

func aclmask(a acl.ACL) os.FileMode {
	for _, entry := range a {
		if entry.Tag == acl.TagMask {
			return entry.Perms
		}
	}

	return 7
}

func find(a acl.ACL, tag acl.Tag, id int, result *os.FileMode) bool {
	q := fmt.Sprintf("%d", id)

	for _, entry := range a {
		if entry.Tag != tag {
			continue
		}

		if entry.Qualifier != q {
			continue
		}

		*result = entry.Perms

		return true
	}

	return false
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
