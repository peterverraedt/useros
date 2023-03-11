package useros

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
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

	errs := []error{
		os.Mkdir(filepath.Join(t.Root, "a"), 0300),
		os.Mkdir(filepath.Join(t.Root, "b"), 0030),
		os.Mkdir(filepath.Join(t.Root, "c"), 0003),
		os.Chmod(filepath.Join(t.Root, "c"), 0003|os.ModeSetgid),
		os.Mkdir(filepath.Join(t.Root, "a", "d"), 0300),
		os.Mkdir(filepath.Join(t.Root, "a", "d", "e"), 0300),
		os.Chown(filepath.Join(t.Root, "a"), 1000, 1000),
		os.Chown(filepath.Join(t.Root, "b"), 1001, 1000),
		os.Chown(filepath.Join(t.Root, "c"), 1001, 1001),
		os.Chown(filepath.Join(t.Root, "a", "d"), 1001, 1000),
		os.Chown(filepath.Join(t.Root, "a", "d", "e"), 1001, 1000),
		os.Symlink(filepath.Join(t.Root, "a", "d"), filepath.Join(t.Root, "d")),
		os.Symlink(filepath.Join(t.Root, "a", "d", "e"), filepath.Join(t.Root, "e")),
	}

	syscall.Umask(m)

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

var i int

func (t *Tree) AssertSuccess(err error) {
	i++

	if err != nil {
		t.T.Errorf("%02d: %s", i, err)
	}
}

func (t *Tree) AssertDenied(err error) {
	i++

	if !os.IsPermission(err) {
		t.T.Errorf("%02d: %v", i, err)
	}
}

func (t *Tree) AssertOwnership(path string, uid int, gid int) {
	i++

	fi, err := os.Stat(path)
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

func (t Tree) Close() {
	os.RemoveAll(t.Root)
}

func Test(t *testing.T) {
	tree := New(t)

	if tree == nil {
		return
	}

	defer tree.Close()

	user1 := User{UID: 1000, GID: 1000}.SedeuidOS()
	user2 := User{UID: 1001, GID: 1000}.SedeuidOS()

	CheckFileWrite(tree, user1, user2)

	user1 = User{UID: 1000, GID: 1000}.OS()
	user2 = User{UID: 1001, GID: 1000}.OS()

	CheckFileWrite(tree, user1, user2)
}

func CheckFileWrite(tree *Tree, user1, user2 OS) {
	root := tree.Root
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "a", "f"), nil, 0644))
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "a", "f"), nil, 0644))
	tree.AssertOwnership(filepath.Join(root, "a", "f"), 1000, 1000)
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "b", "f"), nil, 0644))
	tree.AssertOwnership(filepath.Join(root, "b", "f"), 1000, 1000)
	tree.AssertSuccess(user1.WriteFile(filepath.Join(root, "c", "f"), nil, 0644))
	tree.AssertOwnership(filepath.Join(root, "c", "f"), 1000, 1001)
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "d", "f"), nil, 0644))
	tree.AssertDenied(user2.WriteFile(filepath.Join(root, "d", "e", "f"), nil, 0644))
}

func (u User) SedeuidOS() OS {
	return &def{
		Before: u.setuid,
		After:  u.unsetuid,
	}
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

	setuidLock.Unlock()

	runtime.UnlockOSThread()
}
