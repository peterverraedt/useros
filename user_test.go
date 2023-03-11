package useros

import (
	"runtime"
	"sync"
	"syscall"
	"testing"
)

func Test(t *testing.T) {

}

func (u User) SedeuidOS() OS {
	return &def{
		Before: u.setuid,
		After:  u.unsetuid,
	}
}

var setuidLock sync.Mutex

func (u User) setuid() {
	if syscall.Getuid() > 0 {
		return
	}

	runtime.LockOSThread()

	setuidLock.Lock()

	if u.Gid != syscall.Getegid() {
		if err := syscall.Setegid(u.Gid); err != nil {
			panic(err)
		}
	}

	if u.Uid != syscall.Geteuid() {
		if err := syscall.Seteuid(u.Uid); err != nil {
			panic(err)
		}
	}
}

func (u User) unsetuid() {
	if syscall.Getuid() > 0 {
		return
	}

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
