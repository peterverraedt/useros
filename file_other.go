//go:build !linux
// +build !linux

package virtos

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

func (u *User) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (u *User) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (u *User) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func (u *User) Lchown(name string, uid, gid int) error {
	return os.Lchown(name, uid, gid)
}

func (u *User) Link(oldname, newname string) error {
	return os.Link(oldname, newname)
}

func (u *User) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (u *User) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

func (u *User) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (u *User) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (u *User) Remove(name string) error {
	return os.Remove(name)
}

func (u *User) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (u *User) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (u *User) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (u *User) Truncate(name string, size int64) error {
	return os.Truncate(name, size)
}

func (u *User) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (u *User) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (u *User) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (u *User) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (u *User) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (u *User) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (u *User) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}
