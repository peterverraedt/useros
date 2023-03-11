//go:build linux
// +build linux

package useros

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

func (u User) OS() OS {
	// We should run as root, otherwise return the default os implementation
	if syscall.Geteuid() > 0 {
		return &def{}
	}

	// Assign default values if necessary
	if u.UID < 0 {
		u.UID = syscall.Geteuid()
	}

	if u.GID < 0 {
		u.GID = syscall.Getegid()
	}

	groups, _ := syscall.Getgroups() //nolint:errcheck

	if len(u.Groups) == 0 {
		u.Groups = groups
	}

	// Check whether we are impersonating a user
	if u.UID == syscall.Geteuid() && u.GID == syscall.Getegid() && equal(u.Groups, groups) {
		return &def{}
	}

	return &user{u}
}

func equal(a, b []int) bool {
	for _, k := range a {
		if !contains(b, k) {
			return false
		}
	}

	return len(a) == len(b)
}

func contains(a []int, v int) bool {
	for _, k := range a {
		if k == v {
			return true
		}
	}

	return false
}

type user struct {
	User
}

func (u *user) Chmod(name string, mode os.FileMode) error {
	f, err := u.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return err
	}

	defer f.Close()

	return f.Chmod(mode)
}

func (u *user) Chown(name string, uid, gid int) error {
	f, err := u.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return err
	}

	defer f.Close()

	return f.Chown(uid, gid)
}

func (u *user) Chtimes(name string, atime, mtime time.Time) error {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return err
	}

	if err := u.CheckPermission(name, Write); err != nil {
		return err
	}

	return os.Chtimes(name, atime, mtime)
}

func (u *user) Lchown(name string, uid, gid int) error {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return err
	}

	if err := u.LcheckOwnership(name); err != nil {
		return err
	}

	return os.Lchown(name, uid, gid)
}

// We're not implementing hardlinks
//func (u *user) Link(oldname, newname string) error {
//    return os.ErrPermission
//}

func (u *user) Mkdir(name string, perm os.FileMode) error {
	dirname := filepath.Dir(name)

	// Can execute all parent directories?
	if err := u.canTraverseParents(dirname); err != nil {
		return err
	}

	// Has write permissions
	d, err := os.Open(dirname)
	if err != nil {
		return err
	}

	stat, err := d.Stat()
	if err != nil {
		return err
	}

	// Can create file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return err
	}

	// Create the directory
	if err = os.Mkdir(name, perm); err != nil {
		return err
	}

	return u.chownNewFolderOrSymlink(name, stat)
}

func (u *user) MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := u.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}

		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)

	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i

	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent.
		err = u.MkdirAll(path[:j-1], perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = u.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := u.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}

		return err
	}

	return nil
}

func (u *user) ReadFile(name string) ([]byte, error) {
	f, err := u.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	var size int
	if info, err := f.Stat(); err == nil {
		size64 := info.Size()

		if int64(int(size64)) == size64 {
			size = int(size64)
		}
	}

	size++ // one byte for final read at EOF

	// If a file claims a small size, read at least 512 bytes.
	// In particular, files in Linux's /proc claim size 0 but
	// then do not work right if read in small pieces,
	// so an initial read of 1 byte would not work correctly.
	if size < 512 {
		size = 512
	}

	data := make([]byte, 0, size)

	for {
		if len(data) >= cap(data) {
			d := append(data[:cap(data)], 0)
			data = d[:len(data)]
		}

		n, err := f.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]

		if err != nil {
			if err == io.EOF {
				err = nil
			}

			return data, err
		}
	}
}

func (u *user) Readlink(name string) (string, error) {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return "", err
	}

	return os.Readlink(name)
}

func (u *user) Remove(name string) error {
	dirname := filepath.Dir(name)

	// Has write permissions
	stat, err := u.Stat(dirname)
	if err != nil {
		return err
	}

	// Can delete file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return err
	}

	if stat.Mode()&os.ModeSticky > 0 {
		stat, err = os.Lstat(name)
		if err != nil {
			return err
		}

		if stat.Sys().(*syscall.Stat_t).Uid != uint32(u.UID) && u.UID > 0 {
			return os.ErrPermission
		}
	}

	return os.Remove(name)
}

func (u *user) RemoveAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return nil
	}

	// The rmdir system call does not permit removing ".",
	// so we don't permit it either.
	if endsWithDot(path) {
		return &os.PathError{Op: "RemoveAll", Path: path, Err: syscall.EINVAL}
	}

	// Simple case: if Remove works, we're done.
	err := u.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}

	// Otherwise, is this a directory we need to recurse into?
	dir, serr := u.Lstat(path)
	if serr != nil {
		if serr, ok := serr.(*os.PathError); ok && (os.IsNotExist(serr.Err) || serr.Err == syscall.ENOTDIR) {
			return nil
		}

		return serr
	}

	if !dir.IsDir() {
		// Not a directory; return the error from Remove.
		return err
	}

	// Remove contents & return first error.
	err = nil
	for {
		fd, err := u.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Already deleted by someone else.
				return nil
			}

			return err
		}

		const reqSize = 1024
		var names []string
		var readErr error

		for {
			numErr := 0
			names, readErr = fd.Readdirnames(reqSize)

			for _, name := range names {
				err1 := u.RemoveAll(path + string(os.PathSeparator) + name)
				if err == nil {
					err = err1
				}

				if err1 != nil {
					numErr++
				}
			}

			// If we can delete any entry, break to start new iteration.
			// Otherwise, we discard current names, get next entries and try deleting them.
			if numErr != reqSize {
				break
			}
		}

		// Removing files from the directory may have caused
		// the OS to reshuffle it. Simply calling Readdirnames
		// again may skip some entries. The only reliable way
		// to avoid this is to close and re-open the
		// directory. See issue 20841.
		fd.Close()

		if readErr == io.EOF {
			break
		}

		// If Readdirnames returned an error, use it.
		if err == nil {
			err = readErr
		}

		if len(names) == 0 {
			break
		}

		// We don't want to re-open unnecessarily, so if we
		// got fewer than request names from Readdirnames, try
		// simply removing the directory now. If that
		// succeeds, we are done.
		if len(names) < reqSize {
			err1 := u.Remove(path)
			if err1 == nil || os.IsNotExist(err1) {
				return nil
			}

			if err != nil {
				// We got some error removing the
				// directory contents, and since we
				// read fewer names than we requested
				// there probably aren't more files to
				// remove. Don't loop around to read
				// the directory again. We'll probably
				// just get the same error.
				return err
			}
		}
	}

	// Remove directory.
	err1 := u.Remove(path)
	if err1 == nil || os.IsNotExist(err1) {
		return nil
	}

	if err == nil {
		err = err1
	}

	return err
}

func endsWithDot(path string) bool {
	if path == "." {
		return true
	}
	if len(path) >= 2 && path[len(path)-1] == '.' && os.IsPathSeparator(path[len(path)-2]) {
		return true
	}
	return false
}

func (u *user) Rename(oldpath, newpath string) error {
	dirname := filepath.Dir(oldpath)

	// Has write permissions
	stat, err := u.Stat(dirname)
	if err != nil {
		return err
	}

	// Can delete file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return err
	}

	dirname = filepath.Dir(newpath)

	// Has write permissions
	stat, err = u.Stat(dirname)
	if err != nil {
		return err
	}

	// Can write file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return err
	}

	return os.Rename(oldpath, newpath)
}

func (u *user) Symlink(oldname, newname string) error {
	dirname := filepath.Dir(newname)

	stat, err := u.Stat(dirname)
	if err != nil {
		return err
	}

	// Can write file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return err
	}

	if err = os.Symlink(oldname, newname); err != nil {
		return err
	}

	return u.chownNewFolderOrSymlink(newname, stat)
}

func (u *user) Truncate(name string, size int64) error {
	stat, err := u.Stat(name)
	if err != nil {
		return err
	}

	if err = u.checkPermission(stat, Write); err != nil {
		return err
	}

	return os.Truncate(name, size)
}

func (u *user) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := u.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}

	return err
}

func (u *user) Stat(name string) (os.FileInfo, error) {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return nil, err
	}

	return os.Stat(name)
}

func (u *user) Lstat(name string) (os.FileInfo, error) {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return nil, err
	}

	return os.Lstat(name)
}

func (u *user) Create(name string) (*os.File, error) {
	dirname := filepath.Dir(name)

	stat, err := u.Stat(dirname)
	if err != nil {
		return nil, err
	}

	// Can create file?
	if err = u.checkPermission(stat, Write, Execute); err != nil {
		return nil, err
	}

	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	return f, u.chownNewFile(f, stat)
}

func (u *user) Open(name string) (*os.File, error) {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return nil, err
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}

	return f, u.checkPermission(stat, Read)
}

func (u *user) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	// Can execute all parent directories?
	if err := u.canTraverseParents(name); err != nil {
		return nil, err
	}

	var (
		f   *os.File
		err error
	)

	// Try to open a new file
	for {
		f, err = os.OpenFile(name, flag, perm|fs.FileMode(os.O_EXCL))
		if os.IsExist(err) {
			stat, err := os.Stat(name)
			if os.IsNotExist(err) {
				// The file disappeared in between OpenFile and Stat
				// Retry exclusive OpenFile, it makes no sense to return
				// file not found.
				continue
			} else if err != nil {
				return nil, err
			}

			p := Write

			if perm&fs.FileMode(os.O_RDONLY) > 0 {
				p = Read
			}

			if err = u.checkPermission(stat, p); err != nil {
				return nil, err
			}

			return os.OpenFile(name, flag, perm)
		} else if err != nil {
			return nil, err
		}

		break
	}

	// Check whether we should have created the file

	d, err := os.Open(filepath.Dir(name))
	if err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, err
	}

	stat, err := d.Stat()
	if err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, err
	}

	// Can create file?
	if err = u.checkPermission(stat, Write); err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, err
	}

	if err = u.chownNewFile(f, stat); err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, err
	}

	return f, nil
}

func (u *user) ReadDir(name string) ([]os.DirEntry, error) {
	f, err := u.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	dirs, err := f.ReadDir(-1)
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })

	return dirs, err
}
