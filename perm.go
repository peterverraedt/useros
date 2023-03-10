package virtos

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

type User struct {
	Uid    int
	Gid    int
	Groups []int
}

var ErrTypeAssertion = errors.New("type assertion")

func (u *User) CheckOwnership(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	return u.checkOwnership(stat)
}

func (u *User) LcheckOwnership(name string) error {
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

func (u *User) CheckPermission(name string, permission Permission) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, permission)
}

func (u *User) LcheckPermission(name string, permission Permission) error {
	stat, err := os.Lstat(name)
	if err != nil {
		return err
	}

	return u.checkPermission(stat, permission)
}

// TODO: sticky bit and symlinks
// sudo sysctl fs.protected_symlinks
func (u *User) canTraverseParents(name string) error {
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
