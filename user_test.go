package useros

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

func Test(t *testing.T) {
	// Can only test as root
	if syscall.Geteuid() > 0 || syscall.Getuid() > 0 {
		return
	}

	root, err := os.MkdirTemp("/tmp", "virtos")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(root)

	err = os.Chmod(root, 0755)
	if err != nil {
		t.Fatal(err)
	}

	CreateTree(t, root)

	user1 := User{UID: 1000, GID: 1000}.SedeuidOS()
	user2 := User{UID: 1001, GID: 1000}.SedeuidOS()

	CheckFileWrite(t, root, user1, user2)

	user1 = User{UID: 1000, GID: 1000}.OS()
	user2 = User{UID: 1001, GID: 1000}.OS()

	CheckFileWrite(t, root, user1, user2)
}

func CreateTree(t *testing.T, root string) {
	m := syscall.Umask(0)
	assert(t, os.Mkdir(filepath.Join(root, "a"), 0300))
	assert(t, os.Mkdir(filepath.Join(root, "b"), 0030))
	assert(t, os.Chmod(filepath.Join(root, "b"), 0030|os.ModeSetgid))
	assert(t, os.Mkdir(filepath.Join(root, "c"), 0003))
	assert(t, os.Mkdir(filepath.Join(root, "a", "d"), 0300))
	assert(t, os.Mkdir(filepath.Join(root, "a", "d", "e"), 0300))
	assert(t, os.Chown(filepath.Join(root, "a"), 1000, 1000))
	assert(t, os.Chown(filepath.Join(root, "b"), 1001, 1000))
	assert(t, os.Chown(filepath.Join(root, "c"), 1001, 1001))
	assert(t, os.Chown(filepath.Join(root, "a", "d"), 1001, 1000))
	assert(t, os.Chown(filepath.Join(root, "a", "d", "e"), 1001, 1000))
	assert(t, os.Symlink(filepath.Join(root, "a", "d"), filepath.Join(root, "d")))
	assert(t, os.Symlink(filepath.Join(root, "a", "d", "e"), filepath.Join(root, "e")))
	syscall.Umask(m)
}

func CheckFileWrite(t *testing.T, root string, user1, user2 OS) {
	assertDenied(t, user2.WriteFile(filepath.Join(root, "a", "f"), nil, 0644))
	assert(t, user1.WriteFile(filepath.Join(root, "a", "f"), nil, 0644))
	assert(t, user1.WriteFile(filepath.Join(root, "b", "f"), nil, 0644))
	assert(t, user1.WriteFile(filepath.Join(root, "c", "f"), nil, 0644))
	assertDenied(t, user2.WriteFile(filepath.Join(root, "d", "f"), nil, 0644))
	assertDenied(t, user2.WriteFile(filepath.Join(root, "d", "e", "f"), nil, 0644))
}

var i int

func assert(t *testing.T, err error) {
	i++

	if err != nil {
		t.Fatalf("%02d: %s", i, err)
	}
}

func assertDenied(t *testing.T, err error) {
	i++

	if !os.IsPermission(err) {
		t.Fatalf("%02d: %v", i, err)
	}
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
