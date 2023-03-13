//go:build linux
// +build linux

package useros

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
)

type Tree struct {
	T    *testing.T
	Root string
}

func New(t *testing.T) *Tree {
	if syscall.Geteuid() != 0 || syscall.Getuid() != 0 {
		return nil
	}

	root, err := os.MkdirTemp("/tmp", "virtos")
	if err != nil {
		t.Error(err)
		return nil
	}

	err = os.Chmod(root, 0755)
	if err != nil {
		t.Error(err)
		os.RemoveAll(root) //nolint:errcheck
		return nil
	}

	tree := Tree{
		T:    t,
		Root: root,
	}

	err = tree.CreateTree()
	if err != nil {
		t.Error(err)
		os.RemoveAll(root) //nolint:errcheck
		return nil
	}

	return &tree
}

func (t Tree) CreateTree() error {
	m := syscall.Umask(0)

	d := User{
		GID: 1000,
	}.OS()

	errs := []error{
		d.Mkdir(filepath.Join(t.Root, "a"), 0300),
		d.Mkdir(filepath.Join(t.Root, "b"), 0030),
		d.Mkdir(filepath.Join(t.Root, "c"), 0003),
		d.Chmod(filepath.Join(t.Root, "c"), 0003|os.ModeSetgid),
		d.Mkdir(filepath.Join(t.Root, "a", "d"), 0300),
		d.Mkdir(filepath.Join(t.Root, "a", "d", "e"), 0300),
		d.Chown(filepath.Join(t.Root, "a"), 1000, 1000),
		d.Chown(filepath.Join(t.Root, "b"), 1001, 1000),
		d.Chown(filepath.Join(t.Root, "c"), 1001, 1001),
		d.Chown(filepath.Join(t.Root, "a", "d"), 1001, 1000),
		d.Chown(filepath.Join(t.Root, "a", "d", "e"), 1001, 1000),
		d.Symlink(filepath.Join(t.Root, "a", "d"), filepath.Join(t.Root, "d")),
		d.Symlink(filepath.Join(t.Root, "a", "d", "e"), filepath.Join(t.Root, "e")),
	}

	syscall.Umask(m)

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func (t Tree) Close() {
	os.RemoveAll(t.Root)
}

var i int

func (t Tree) AssertSuccess(err error) {
	i++

	if err != nil {
		t.T.Errorf("%02d: %s", i, err)
	}
}

func (t Tree) AssertDenied(err error) {
	i++

	if err == nil {
		t.T.Errorf("%02d: succeeded but expected access denied", i)
		return
	}

	if !os.IsPermission(err) {
		t.T.Errorf("%02d: %s", i, err)
	}
}

func (t Tree) AssertNotExist(err error) {
	i++

	if err == nil {
		t.T.Errorf("%02d: succeeded but expected no file", i)
		return
	}

	if !os.IsNotExist(err) {
		t.T.Errorf("%02d: %s", i, err)
	}
}

func (t Tree) AssertNotDir(err error) {
	i++

	if err == nil {
		t.T.Errorf("%02d: succeeded but expected no dir", i)
		return
	}

	if !errors.Is(err, syscall.ENOTDIR) {
		t.T.Errorf("%02d: %s", i, err)
	}
}

func (t Tree) AssertOwnership(path string, uid int, gid int) {
	i++

	fi, err := os.Lstat(path)
	if err != nil {
		t.T.Errorf("%02d: %v", i, err)
		return
	}

	stat_t, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.T.Errorf("%02d: assertion error", i)
		return
	}

	if stat_t.Uid != uint32(uid) || stat_t.Gid != uint32(gid) {
		t.T.Errorf("%02d: invalid ownership", i)
	}
}

func (t Tree) AssertContent(path string, body []byte) {
	i++

	content, err := os.ReadFile(path)
	if err != nil {
		t.T.Errorf("%02d: %v", i, err)
		return
	}

	if !bytes.Equal(content, body) {
		t.T.Errorf("%02d: invalid content", i)
		return
	}
}

func (t *Tree) Test(c func(tree Tree)) {
	if t == nil {
		return
	}

	defer t.Close()

	c(*t)
}

func TestFileWrite(t *testing.T) {
	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000}.SedeuidOS()
		user2 := User{UID: 1001, GID: 1000}.SedeuidOS()

		CheckFileWrite(tree, user1, user2)
	})

	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000}.OS()
		user2 := User{UID: 1001, GID: 1000}.OS()

		CheckFileWrite(tree, user1, user2)
	})
}

func CheckFileWrite(tree Tree, user1, user2 OS) {
	body := []byte("hello")
	root := tree.Root
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "a", "f"), body, 0600))
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "a", "f"), body, 0600))
	tree.AssertContent(filepath.Join(root, "a", "f"), body)
	_, err := user1.ReadFile(filepath.Join(root, "a", "f"))
	tree.AssertSuccess(err)
	_, err = user2.ReadFile(filepath.Join(root, "a", "f"))
	tree.AssertDenied(err)
	tree.AssertContent(filepath.Join(root, "a", "f"), body)
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "a", "f"), nil, 0600))
	tree.AssertContent(filepath.Join(root, "a", "f"), body)
	tree.AssertOwnership(filepath.Join(root, "a", "f"), 1000, 1000)
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "b", "f"), body, 0640))
	_, err = user2.ReadFile(filepath.Join(root, "b", "f"))
	tree.AssertDenied(err)
	tree.AssertOwnership(filepath.Join(root, "b", "f"), 1000, 1000)
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "c", "f"), body, 0600))
	tree.AssertOwnership(filepath.Join(root, "c", "f"), 1000, 1001)
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "d", "f"), body, 0600))
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "d", "e", "f"), body, 0600))
	_, err = user2.Create(filepath.Join(root, "a", "g"))
	tree.AssertDenied(err)

	f, err := user1.Create(filepath.Join(root, "a", "g"))
	if f != nil {
		f.Close()
	}
	tree.AssertSuccess(err)

	f, err = user1.OpenFile(filepath.Join(root, "a", "h"), syscall.O_RDONLY, 0755)
	if f != nil {
		f.Close()
	}
	tree.AssertNotExist(err)

	f, err = user1.Open(filepath.Join(root, "a", "h"))
	if f != nil {
		f.Close()
	}
	tree.AssertNotExist(err)

	f, err = user1.OpenFile(filepath.Join(root, "a", "h"), syscall.O_RDONLY|syscall.O_CREAT, 0755)
	if f != nil {
		f.Close()
	}
	tree.AssertSuccess(err)
}

func TestRemove(t *testing.T) {
	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000}.SedeuidOS()
		user2 := User{UID: 1002, GID: 1000}.SedeuidOS()

		CheckRemove(tree, user1, user2)
	})

	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000}.OS()
		user2 := User{UID: 1002, GID: 1000}.OS()

		CheckRemove(tree, user1, user2)
	})
}

func CheckRemove(tree Tree, user1, user2 OS) {
	body := []byte("hello")
	path := filepath.Join(tree.Root, "b", "f")
	tree.AssertSuccess(user1.WriteFile(path, body, 0600))
	tree.AssertOwnership(path, 1000, 1000)
	tree.AssertDenied(user2.Truncate(path, 2))
	tree.AssertContent(path, body)
	tree.AssertSuccess(user1.Truncate(path, 2))
	tree.AssertContent(path, []byte("he"))
	tree.AssertSuccess(user2.Remove(path))
	tree.AssertSuccess(os.Chmod(filepath.Join(tree.Root, "b"), 0030|os.ModeSticky))
	tree.AssertSuccess(user1.WriteFile(path, nil, 0600))
	tree.AssertOwnership(path, 1000, 1000)
	tree.AssertDenied(user2.Remove(path))
	tree.AssertSuccess(user1.Remove(path))
}

func TestChownChmod(t *testing.T) {
	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000, Groups: []int{1001}}.SedeuidOS()
		user2 := User{UID: 1001, GID: 1000}.SedeuidOS()

		CheckChownChmod(tree, user1, user2)
	})

	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000, Groups: []int{1001}}.OS()
		user2 := User{UID: 1001, GID: 1000}.OS()

		CheckChownChmod(tree, user1, user2)
	})
}

func CheckChownChmod(tree Tree, user1, user2 OS) {
	path := filepath.Join(tree.Root, "a", "f")
	tree.AssertNotExist(user1.Chown(path, 1000, 1001))
	tree.AssertNotExist(user1.Chmod(path, 0644))
	tree.AssertSuccess(user1.WriteFile(path, nil, 0600))
	tree.AssertOwnership(path, 1000, 1000)
	tree.AssertSuccess(user1.Chown(path, 1000, 1001))
	tree.AssertOwnership(path, 1000, 1001)
	tree.AssertDenied(user1.Chown(path, 1001, 1000))
	tree.AssertSuccess(user1.Chmod(path, 0644))
	tree.AssertSuccess(user1.Chtimes(path, time.Time{}, time.Time{}))
	tree.AssertDenied(user2.Chmod(path, 0644))
	tree.AssertDenied(user2.Chtimes(path, time.Time{}, time.Time{}))
	_, err := user1.Stat(path)
	tree.AssertSuccess(err)
	_, err = user2.Stat(path)
	tree.AssertDenied(err)

	link := filepath.Join(tree.Root, "a", "s")
	tree.AssertSuccess(user1.Symlink("f", link))
	tree.AssertOwnership(link, 1000, 1000)
	tree.AssertOwnership(path, 1000, 1001)
	tree.AssertSuccess(user1.Chown(path, 1000, 1000))
	tree.AssertOwnership(link, 1000, 1000)
	tree.AssertOwnership(path, 1000, 1000)
	tree.AssertSuccess(user1.Lchown(link, 1000, 1001))
	tree.AssertOwnership(link, 1000, 1001)
	tree.AssertOwnership(path, 1000, 1000)
	_, err = user1.Readlink(link)
	tree.AssertSuccess(err)
	_, err = user2.Readlink(link)
	tree.AssertDenied(err)
	_, err = user1.Stat(link)
	tree.AssertSuccess(err)
	_, err = user1.Lstat(link)
	tree.AssertSuccess(err)
	_, err = user2.Lstat(path)
	tree.AssertDenied(err)
	tree.AssertSuccess(user1.Rename(link, path))
}

func TestMkdir(t *testing.T) {
	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000, Groups: []int{1001}}.SedeuidOS()
		user2 := User{UID: 1002, GID: 1000}.SedeuidOS()

		CheckMkdir(tree, user1, user2)
	})

	New(t).Test(func(tree Tree) {
		user1 := User{UID: 1000, GID: 1000, Groups: []int{1001}}.OS()
		user2 := User{UID: 1002, GID: 1000}.OS()

		CheckMkdir(tree, user1, user2)
	})
}

func CheckMkdir(tree Tree, user1, user2 OS) {
	path := filepath.Join(tree.Root, "a", "x")
	tree.AssertSuccess(user1.Mkdir(path, 0740))
	tree.AssertOwnership(path, 1000, 1000)
	tree.AssertSuccess(user1.WriteFile(filepath.Join(path, "f"), nil, 0600))
	_, err := user1.ReadDir(path)
	tree.AssertSuccess(err)
	// The following check should succeed as rm -rf <path> works, but golang native implementation denies it.
	//tree.AssertSuccess(user1.RemoveAll(path))
	tree.AssertSuccess(user1.Chmod(filepath.Join(tree.Root, "a"), 0700))
	tree.AssertSuccess(user1.RemoveAll(path))

	path = filepath.Join(tree.Root, "a", "x", "y", "z", "t")
	tree.AssertDenied(user2.MkdirAll(path, 0700))
	tree.AssertSuccess(user1.MkdirAll(path, 0700))

	err = user1.Walk(path, func(path string, info fs.FileInfo, err error) error { return err })
	tree.AssertSuccess(err)

	err = user2.Walk(path, func(path string, info fs.FileInfo, err error) error { return err })
	tree.AssertDenied(err)

	path = filepath.Join(tree.Root, "b", "x")
	tree.AssertSuccess(user1.Mkdir(path, 0777))
	_, err = user2.ReadDir(path)
	tree.AssertSuccess(err)
}

func (u User) SedeuidOS() OS {
	return &def{
		Before: u.setuid,
		After:  u.unsetuid,
	}
}

func TestCoverage(t *testing.T) {
	if Default() == nil {
		t.Error("default is nil")
	}

	New(t).Test(func(tree Tree) {
		user1 := User{GID: 1000}.OS()
		user2 := User{UID: 1002, GID: 1000}.OS()

		tree.AssertNotExist(user1.Mkdir(filepath.Join(tree.Root, "i", "do", "not", "exist"), 0755))
		tree.AssertSuccess(user1.WriteFile(filepath.Join(tree.Root, "i"), nil, 0600))
		tree.AssertNotDir(user1.Mkdir(filepath.Join(tree.Root, "i", "do"), 0755))

		_, err := user2.EvalSymlinks(filepath.Join(tree.Root, "d", "e"))
		tree.AssertDenied(err)
	})
}

var setuidLock sync.Mutex

func (u User) setuid() {
	runtime.LockOSThread()

	setuidLock.Lock()

	if u.GID != syscall.Getegid() {
		if err := syscall.Setegid(u.GID); err != nil {
			panic(err)
		}
	}

	if err := syscall.Setgroups(u.Groups); err != nil {
		panic(err)
	}

	if u.UID != syscall.Geteuid() {
		if err := syscall.Seteuid(u.UID); err != nil {
			panic(err)
		}
	}
}

func (u User) unsetuid() {
	if syscall.Getuid() != syscall.Geteuid() {
		if err := syscall.Seteuid(syscall.Getuid()); err != nil {
			panic(err)
		}
	}

	if syscall.Getgid() != syscall.Getegid() {
		if err := syscall.Setegid(syscall.Getgid()); err != nil {
			panic(err)
		}
	}

	if err := syscall.Setgroups(nil); err != nil {
		panic(err)
	}

	setuidLock.Unlock()

	runtime.UnlockOSThread()
}
