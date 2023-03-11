package useros

import (
	"os"
	"time"
)

type def struct{}

func (*def) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}

func (*def) Chown(name string, uid, gid int) error {
	return os.Chown(name, uid, gid)
}

func (*def) Chtimes(name string, atime, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
}

func (*def) Lchown(name string, uid, gid int) error {
	return os.Lchown(name, uid, gid)
}

func (*def) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

func (*def) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (*def) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (*def) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (*def) Remove(name string) error {
	return os.Remove(name)
}

func (*def) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (*def) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (*def) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (*def) Truncate(name string, size int64) error {
	return os.Truncate(name, size)
}

func (*def) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (*def) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (*def) Lstat(name string) (os.FileInfo, error) {
	return os.Lstat(name)
}

func (*def) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (*def) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (*def) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (*def) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}
