package useros

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/joshlf/go-acl"
)

// returns: stat of parent dir, error
func (u User) hasInodeAccess(name string, perm Permission) (os.FileInfo, acl.ACL, error) {
	// Root always has access if the directory exists
	if u.UID == 0 {
		stat, err := os.Stat(filepath.Dir(name))
		if err != nil {
			return nil, nil, err
		}

		if !stat.IsDir() {
			return nil, nil, syscall.ENOTDIR
		}

		a, err := acl.Get(filepath.Dir(name))

		return stat, a, err
	}

	dirs, err := Directories(name)
	if err != nil {
		return nil, nil, err
	}

	checked := make(map[string]struct{})

	var (
		stat fs.FileInfo
		a    acl.ACL
	)

	for i, dir := range dirs {
		// Do not double check directories, but always check the last one
		if _, ok := checked[dir]; ok && i+1 < len(dirs) {
			continue
		}

		stat, err = os.Stat(dir)
		if err != nil {
			return nil, nil, err
		}

		if !stat.IsDir() {
			return nil, nil, syscall.ENOTDIR
		}

		a, err = acl.Get(dir)
		if err != nil {
			return nil, nil, err
		}

		err = u.checkPermission(stat, a, Execute)
		if err != nil {
			return nil, nil, err
		}

		checked[dir] = struct{}{}
	}

	// Check the last directory (directory of the inode) for the asked permission
	if perm == Read || perm == Write {
		return stat, nil, u.checkPermission(stat, a, perm)
	}

	return stat, a, nil
}

func (u User) gidForNewFiles(parent os.FileInfo) int {
	if parent.Mode()&os.ModeSetgid == 0 {
		return u.GID
	}

	stat_t, ok := parent.Sys().(*syscall.Stat_t)
	if !ok {
		return u.GID
	}

	return int(stat_t.Gid)
}

func (u User) hasObjectAccess(name string, perm Permission) error {
	// Root always has access if the object exists
	if u.UID == 0 {
		_, err := os.Stat(name)
		return err
	}

	if _, _, err := u.hasInodeAccess(name, Execute); err != nil {
		return err
	}

	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	a, err := acl.Get(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, a, perm)
}

func (u User) owns(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	return u.checkOwnership(stat)
}

func (u User) lowns(name string) error {
	stat, err := os.Lstat(name)
	if err != nil {
		return err
	}

	return u.checkOwnership(stat)
}

func (u User) chownNewFile(name string, gid int) error {
	return os.Lchown(name, u.UID, gid)
}

func (u User) checkPermission(stat os.FileInfo, a acl.ACL, perms ...Permission) error {
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
			access = perm.Check(stat.Mode() >> 6)
		case find(a, acl.TagUser, u.UID, &aclmode):
			access = perm.Check(aclmode & aclmask(a))
		default:
			foundGroup := false

			mask := aclmask(a)

			for _, g := range groups {
				if stat_t.Gid == uint32(g) {
					access = access || perm.Check(stat.Mode()>>3&mask)
					foundGroup = true
				}

				if find(a, acl.TagGroup, u.GID, &aclmode) {
					access = access || perm.Check(aclmode&mask)
					foundGroup = true
				}
			}

			if !foundGroup {
				access = perm.Check(stat.Mode())
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

func (u User) checkOwnership(stat fs.FileInfo) error {
	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	if stat_t.Uid != uint32(u.UID) && u.UID > 0 {
		return os.ErrPermission
	}

	return nil
}
