//go:build !linux
// +build !linux

package useros

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/joshlf/go-acl"
)

func (u User) hasInodeAccess(name string, perm Permission) (os.FileInfo, acl.ACL, error) {
	stat, err := os.Stat(filepath.Dir(name))
	if err != nil {
		return nil, nil, err
	}

	if !stat.IsDir() {
		return nil, nil, syscall.ENOTDIR
	}

	return stat, nil, nil
}

func (u User) checkDirExecuteOnly(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		return syscall.ENOTDIR
	}

	return nil
}

func (u User) hasObjectAccess(name string, perm Permission) error {
	_, err := os.Stat(name)
	return err
}

func (u User) owns(name string) error {
	_, err := os.Stat(name)
	return err
}

func (u User) lowns(name string) error {
	_, err := os.Lstat(name)
	return err
}

func (u User) gidForNewFiles(parent os.FileInfo) int {
	return 0
}

func (u User) chownNewFile(name string, gid int) error {
	return nil
}

func (u User) checkPermission(stat os.FileInfo, a acl.ACL, perms ...Permission) error {
	return nil
}
