//go:build !linux
// +build !linux

package useros

func (u *user) hasInodeAccess(name string, perm Permission) (int, acl.ACL, error) {
	stat, err := os.Stat(filepath.Dir(name))
	if err != nil {
		return 0, nil, err
	}

	if !stat.IsDir() {
		return 0, nil, syscall.ENOTDIR
	}

	return 0, nil, nil
}

func (u *user) hasObjectAccess(name string, perm Permission) error {
	_, err := os.Stat(name)
	return err
}

func (u User) owns(name string) error {
	_, err := os.Stat(name)
	return err
}

func (u User) lowns(name string) error {
	_, err := os.Lstat(name)
	return err
}

func (u User) gidForNewFiles(parent os.FileInfo) int {
	return 0
}

func (u User) chownNewFile(name string, gid int) error {
	return nil
}

func (u User) checkPermission(stat os.FileInfo, a acl.ACL, perms ...Permission) error {
	return nil
}
