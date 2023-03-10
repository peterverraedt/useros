package virtos

import "os"

func (u *User) Chmod(name string, mode os.FileMode) error {
	if err := u.CheckOwnership(name); err != nil {
		return err
	}

	return os.Chmod(name, mode)
}

func (u *User) Chown(name string, uid, gid int) error {
	if err := u.CheckOwnership(name); err != nil {
		return err
	}

	return os.Chown(name, uid, gid)
}
