package virtos

import (
	"errors"
	"os"
	"syscall"
)

var ErrTypeAssertion = errors.New("type assertion")

func (u *User) CheckOwnership(name string) error {
	stat, err := os.Stat(name)
	if err != nil {
		return err
	}

	stat_t, ok := stat.Sys().(*syscall.Stat_t)
	if !ok {
		return ErrTypeAssertion
	}

	if stat_t.Uid != uint32(u.Uid) {
		return os.ErrPermission
	}

	return nil
}
