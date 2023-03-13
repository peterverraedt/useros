package useros

import (
	"io"
	"io/fs"
	"os"
	"sort"
	"syscall"
	"time"

	"github.com/joshlf/go-acl"
)

type User struct {
	UID    int
	GID    int
	Groups []int
}

type OS interface {
	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Chtimes(name string, atime, mtime time.Time) error
	Lchown(name string, uid, gid int) error
	Mkdir(name string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(name string) ([]byte, error)
	Readlink(name string) (string, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath, newpath string) error
	Symlink(oldname, newname string) error
	Truncate(name string, size int64) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)
	Create(name string) (*os.File, error)
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

// OS returns a simulated version of os as if the user would run the commands.
func (u User) OS() OS {
	return u.os()
}

// CanWriteInode checks whether the user can write to the file inode at the specified path.
// The inode itself doesn't have to exist, its parent directory should exist.
// If nil is returned, the inode is writable by the user.
func (u User) CanWriteInode(path string) error {
	_, _, err := u.hasInodeAccess(path, Write)
	return err
}

// CanReadInode checks whether the user can read the file inode at the specified path,
// i.e. whether the inode can be statted (= execution bit on the parent directory).
// The inode itself doesn't have to exist, its parent directory should exist.
// If nil is returned, the inode is readable by the user.
func (u User) CanReadInode(path string) error {
	_, _, err := u.hasInodeAccess(path, Execute)
	return err
}

// CanWriteObject checks whether the user can write to the file or directory at the specified path.
// The file or directory needs to exist, as its permission bits are checked. Symlinks are followed.
// If nil is returned, the object is writable by the user.
func (u User) CanWriteObject(path string) error {
	return u.hasObjectAccess(path, Write)
}

// CanReadObject checks whether the user can read the file or directory at the specified path.
// The file or directory needs to exist, as its permission bits are checked. Symlinks are followed.
// If nil is returned, the object is readable by the user.
func (u User) CanReadObject(path string) error {
	return u.hasObjectAccess(path, Read)
}

// Owns checks whether the user owns the file or directory at the specified path.
// The file or directory needs to exist, as its owner is checked. Symlinks are followed.
// If nil is returned, the object is owned by the user.
func (u User) Owns(path string) error {
	return u.owns(path)
}

// Lowns checks whether the user owns the file or directory at the specified path.
// The file or directory needs to exist, as its owner is checked. Symlinks are not followed.
// If nil is returned, the object is owned by the user.
func (u User) Lowns(path string) error {
	return u.lowns(path)
}

type user struct {
	User
}

func (u *user) Chmod(name string, mode os.FileMode) error {
	if err := u.CanReadInode(name); err != nil {
		return err
	}

	if err := u.Owns(name); err != nil {
		return err
	}

	return os.Chmod(name, mode)
}

func (u *user) Chown(name string, uid, gid int) error {
	if err := u.CanReadInode(name); err != nil {
		return err
	}

	if err := u.Owns(name); err != nil {
		return err
	}

	if u.UID == 0 {
		return os.Chown(name, uid, gid)
	}

	if uid != u.UID {
		return os.ErrPermission
	}

	if gid != u.GID && !contains(u.Groups, gid) {
		return os.ErrPermission
	}

	return os.Chown(name, uid, gid)
}

// TODO: check permission checks
func (u *user) Chtimes(name string, atime, mtime time.Time) error {
	if err := u.CanReadInode(name); err != nil {
		return err
	}

	if err := u.CanWriteObject(name); err != nil {
		return err
	}

	return os.Chtimes(name, atime, mtime)
}

func (u *user) Lchown(name string, uid, gid int) error {
	if err := u.CanReadInode(name); err != nil {
		return err
	}

	if err := u.Lowns(name); err != nil {
		return err
	}

	if u.UID == 0 {
		return os.Lchown(name, uid, gid)
	}

	if uid != u.UID {
		return os.ErrPermission
	}

	if gid != u.GID && !contains(u.Groups, gid) {
		return os.ErrPermission
	}

	return os.Lchown(name, uid, gid)
}

func (u *user) Mkdir(name string, perm os.FileMode) error {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return err
	}

	if err = os.Mkdir(name, perm); err != nil {
		return err
	}

	return u.chownNewFile(name, u.gidForNewFiles(stat))
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
	if err := u.CanReadInode(name); err != nil {
		return "", err
	}

	return os.Readlink(name)
}

func (u *user) Remove(name string) error {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return err
	}

	if stat.Mode()&os.ModeSticky > 0 {
		err = u.Lowns(name)
		if err != nil {
			return err
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
	if err := u.CanWriteInode(oldpath); err != nil {
		return err
	}

	if err := u.CanWriteInode(newpath); err != nil {
		return err
	}

	return os.Rename(oldpath, newpath)
}

func (u *user) Symlink(oldname, newname string) error {
	stat, _, err := u.hasInodeAccess(newname, Write)
	if err != nil {
		return err
	}

	if err = os.Symlink(oldname, newname); err != nil {
		return err
	}

	return u.chownNewFile(newname, u.gidForNewFiles(stat))
}

func (u *user) Truncate(name string, size int64) error {
	stat, err := u.Stat(name)
	if err != nil {
		return err
	}

	a, err := acl.Get(name)
	if err != nil {
		return err
	}

	if err = u.checkPermission(stat, a, Write); err != nil {
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
	if err := u.CanReadInode(name); err != nil {
		return nil, err
	}

	return os.Stat(name)
}

func (u *user) Lstat(name string) (os.FileInfo, error) {
	if err := u.CanReadInode(name); err != nil {
		return nil, err
	}

	return os.Lstat(name)
}

func (u *user) Create(name string) (*os.File, error) {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	err = u.chownNewFile(name, u.gidForNewFiles(stat))
	if err != nil {
		f.Close()
		os.Remove(name) //nolint:errcheck

		return nil, err
	}

	return f, nil
}

func (u *user) Open(name string) (*os.File, error) {
	if err := u.CanReadObject(name); err != nil {
		return nil, err
	}

	return os.Open(name)
}

func (u *user) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	stat, a, err := u.hasInodeAccess(name, Execute)
	if err != nil {
		return nil, err
	}

	var f *os.File

	// Try to open a new file
	for {
		f, err = os.OpenFile(name, flag, perm|fs.FileMode(os.O_EXCL))
		if os.IsExist(err) {
			p := Write

			if perm&fs.FileMode(os.O_RDONLY) > 0 {
				p = Read
			}

			err := u.hasObjectAccess(name, p)
			if os.IsNotExist(err) {
				// The file disappeared in between OpenFile and Stat
				// Retry exclusive OpenFile, it makes no sense to return
				// file not found.
				continue
			} else if err != nil {
				return nil, err
			}

			return os.OpenFile(name, flag, perm)
		} else if err != nil {
			return nil, err
		}

		break
	}

	// Can create file?
	if err = u.checkPermission(stat, a, Write); err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, err
	}

	if err = u.chownNewFile(name, u.gidForNewFiles(stat)); err != nil {
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
