package useros

import (
	"os"
	"time"
)

// Default OS implementation is a simple wrapper around the pkg.go.dev/os functions.
// Mainly usefull in situations where conditionally the default OS or a user view to the OS is needed.
func Default() OS {
	return &def{}
}

type def struct {
	Before, After func()
}

func (d *def) Chmod(name string, mode os.FileMode) error {
	d.wrap()
	defer d.unwrap()
	return os.Chmod(name, mode)
}

func (d *def) Chown(name string, uid, gid int) error {
	d.wrap()
	defer d.unwrap()
	return os.Chown(name, uid, gid)
}

func (d *def) Chtimes(name string, atime, mtime time.Time) error {
	d.wrap()
	defer d.unwrap()
	return os.Chtimes(name, atime, mtime)
}

func (d *def) Lchown(name string, uid, gid int) error {
	d.wrap()
	defer d.unwrap()
	return os.Lchown(name, uid, gid)
}

func (d *def) Mkdir(name string, perm os.FileMode) error {
	d.wrap()
	defer d.unwrap()
	return os.Mkdir(name, perm)
}

func (d *def) MkdirAll(path string, perm os.FileMode) error {
	d.wrap()
	defer d.unwrap()
	return os.MkdirAll(path, perm)
}

func (d *def) ReadFile(name string) ([]byte, error) {
	d.wrap()
	defer d.unwrap()
	return os.ReadFile(name)
}

func (d *def) Readlink(name string) (string, error) {
	d.wrap()
	defer d.unwrap()
	return os.Readlink(name)
}

func (d *def) Remove(name string) error {
	d.wrap()
	defer d.unwrap()
	return os.Remove(name)
}

func (d *def) RemoveAll(path string) error {
	d.wrap()
	defer d.unwrap()
	return os.RemoveAll(path)
}

func (d *def) Rename(oldpath, newpath string) error {
	d.wrap()
	defer d.unwrap()
	return os.Rename(oldpath, newpath)
}

func (d *def) Symlink(oldname, newname string) error {
	d.wrap()
	defer d.unwrap()
	return os.Symlink(oldname, newname)
}

func (d *def) Truncate(name string, size int64) error {
	d.wrap()
	defer d.unwrap()
	return os.Truncate(name, size)
}

func (d *def) WriteFile(name string, data []byte, perm os.FileMode) error {
	d.wrap()
	defer d.unwrap()
	return os.WriteFile(name, data, perm)
}

func (d *def) Stat(name string) (os.FileInfo, error) {
	d.wrap()
	defer d.unwrap()
	return os.Stat(name)
}

func (d *def) Lstat(name string) (os.FileInfo, error) {
	d.wrap()
	defer d.unwrap()
	return os.Lstat(name)
}

func (d *def) Create(name string) (*os.File, error) {
	d.wrap()
	defer d.unwrap()
	return os.Create(name)
}

func (d *def) Open(name string) (*os.File, error) {
	d.wrap()
	defer d.unwrap()
	return os.Open(name)
}

func (d *def) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	d.wrap()
	defer d.unwrap()
	return os.OpenFile(name, flag, perm)
}

func (d *def) ReadDir(name string) ([]os.DirEntry, error) {
	d.wrap()
	defer d.unwrap()
	return os.ReadDir(name)
}

func (d *def) wrap() {
	if d.Before != nil {
		d.Before()
	}
}

func (d *def) unwrap() {
	if d.After != nil {
		d.After()
	}
}
