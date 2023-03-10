package virtos

import (
	"io/fs"
	"os"
	"syscall"
)

func (u *User) checkOwnership(stat fs.FileInfo) error {
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
func (u *User) checkPermission(stat os.FileInfo, perms ...Permission) error {
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

func (u *User) chownNewFile(f *os.File, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return f.Chown(u.Uid, u.Gid)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return f.Chown(u.Uid, int(stat_t.Gid))
}

func (u *User) chownNewFolderOrSymlink(name string, parent os.FileInfo) error {
	if parent.Mode()&os.ModeSetgid == 0 {
		return os.Lchown(name, u.Uid, u.Gid)
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	return os.Lchown(name, u.Uid, int(stat_t.Gid))
}
