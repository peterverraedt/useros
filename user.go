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

type User struct {
	UID    int
	GID    int
	Groups []int
}

type OS interface {
	CurrentUser() User
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
	Create(name string) (File, error)
	Open(name string) (File, error)
	OpenFile(name string, flag int, perm os.FileMode) (File, error)
	ReadDir(name string) ([]os.DirEntry, error)
	EvalSymlinks(name string) (string, error)
	Walk(name string, walkFn filepath.WalkFunc) error
}

type File interface {
	Chdir() error
	Chmod(os.FileMode) error
	Chown(uid, gid int) error
	Close() error
	Fd() uintptr
	Name() string
	Read(b []byte) (int, error)
	ReadAt(b []byte, off int64) (int, error)
	ReadDir(n int) ([]os.DirEntry, error)
	ReadFrom(r io.Reader) (int64, error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Seek(offset int64, whence int) (int64, error)
	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Stat() (os.FileInfo, error)
	Sync() error
	SyscallConn() (syscall.RawConn, error)
	Truncate(size int64) error
	Write(b []byte) (int, error)
	WriteAt(b []byte, off int64) (int, error)
	WriteString(s string) (int, error)
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
	return logit(err)
}

// CanReadInode checks whether the user can read the file inode at the specified path,
// i.e. whether the inode can be statted (= execution bit on the parent directory).
// The inode itself doesn't have to exist, its parent directory should exist.
// If nil is returned, the inode is readable by the user.
func (u User) CanReadInode(path string) error {
	_, _, err := u.hasInodeAccess(path, Execute)
	return logit(err)
}

// CanWriteObject checks whether the user can write to the file or directory at the specified path.
// The file or directory needs to exist, as its permission bits are checked. Symlinks are followed.
// If nil is returned, the object is writable by the user.
func (u User) CanWriteObject(path string) error {
	return logit(u.hasObjectAccess(path, Write))
}

// CanReadObject checks whether the user can read the file or directory at the specified path.
// The file or directory needs to exist, as its permission bits are checked. Symlinks are followed.
// If nil is returned, the object is readable by the user.
func (u User) CanReadObject(path string) error {
	return logit(u.hasObjectAccess(path, Read))
}

// Owns checks whether the user owns the file or directory at the specified path.
// The file or directory needs to exist, as its owner is checked. Symlinks are followed.
// If nil is returned, the object is owned by the user.
func (u User) Owns(path string) error {
	return logit(u.owns(path))
}

// Lowns checks whether the user owns the file or directory at the specified path.
// The file or directory needs to exist, as its owner is checked. Symlinks are not followed.
// If nil is returned, the object is owned by the user.
func (u User) Lowns(path string) error {
	return logit(u.lowns(path))
}

type user struct {
	User
}

func (u *user) CurrentUser() User {
	return u.User
}

func (u *user) Chmod(name string, mode os.FileMode) error {
	if err := u.CanReadInode(name); err != nil {
		return logit(err)
	}

	if err := u.Owns(name); err != nil {
		return logit(err)
	}

	return logit(os.Chmod(name, mode))
}

func (u *user) Chown(name string, uid, gid int) error {
	if err := u.CanReadInode(name); err != nil {
		return logit(err)
	}

	if err := u.Owns(name); err != nil {
		return logit(err)
	}

	if u.UID == 0 {
		return logit(os.Chown(name, uid, gid))
	}

	if uid != u.UID {
		return logit(os.ErrPermission)
	}

	if gid != u.GID && !contains(u.Groups, gid) {
		return logit(os.ErrPermission)
	}

	return logit(os.Chown(name, uid, gid))
}

// TODO: check permission checks
func (u *user) Chtimes(name string, atime, mtime time.Time) error {
	if err := u.CanReadInode(name); err != nil {
		return logit(err)
	}

	if err := u.CanWriteObject(name); err != nil {
		return logit(err)
	}

	return logit(os.Chtimes(name, atime, mtime))
}

func (u *user) Lchown(name string, uid, gid int) error {
	if err := u.CanReadInode(name); err != nil {
		return logit(err)
	}

	if err := u.Lowns(name); err != nil {
		return logit(err)
	}

	if u.UID == 0 {
		return logit(os.Lchown(name, uid, gid))
	}

	if uid != u.UID {
		return logit(os.ErrPermission)
	}

	if gid != u.GID && !contains(u.Groups, gid) {
		return logit(os.ErrPermission)
	}

	return logit(os.Lchown(name, uid, gid))
}

func (u *user) Mkdir(name string, perm os.FileMode) error {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return logit(err)
	}

	if err = os.Mkdir(name, perm); err != nil {
		return logit(err)
	}

	return logit(u.chownNewFile(name, u.gidForNewFiles(stat)))
}

func (u *user) MkdirAll(path string, perm os.FileMode) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := u.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return logit(nil)
		}

		return logit(&os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR})
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
			return logit(err)
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = u.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := u.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return logit(nil)
		}

		return logit(err)
	}

	return logit(nil)
}

func (u *user) ReadFile(name string) ([]byte, error) {
	f, err := u.Open(name)
	if err != nil {
		return nil, logit(err)
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

			return data, logit(err)
		}
	}
}

func (u *user) Readlink(name string) (string, error) {
	if err := u.CanReadInode(name); err != nil {
		return "", logit(err)
	}

	r, err := os.Readlink(name)

	return r, logit(err)
}

func (u *user) Remove(name string) error {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return logit(err)
	}

	if stat.Mode()&os.ModeSticky > 0 {
		err = u.Lowns(name)
		if err != nil {
			return logit(err)
		}
	}

	return logit(os.Remove(name))
}

func (u *user) RemoveAll(path string) error {
	if path == "" {
		// fail silently to retain compatibility with previous behavior
		// of RemoveAll. See issue 28830.
		return logit(nil)
	}

	// The rmdir system call does not permit removing ".",
	// so we don't permit it either.
	if endsWithDot(path) {
		return logit(&os.PathError{Op: "RemoveAll", Path: path, Err: syscall.EINVAL})
	}

	// Simple case: if Remove works, we're done.
	err := u.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return logit(nil)
	}

	// Otherwise, is this a directory we need to recurse into?
	dir, serr := u.Lstat(path)
	if serr != nil {
		if serr, ok := serr.(*os.PathError); ok && (os.IsNotExist(serr.Err) || serr.Err == syscall.ENOTDIR) {
			return logit(nil)
		}

		return logit(serr)
	}

	if !dir.IsDir() {
		// Not a directory; return the error from Remove.
		return logit(err)
	}

	// Remove contents & return first error.
	err = nil
	for {
		fd, err := u.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Already deleted by someone else.
				return logit(nil)
			}

			return logit(err)
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
				return logit(nil)
			}

			if err != nil {
				// We got some error removing the
				// directory contents, and since we
				// read fewer names than we requested
				// there probably aren't more files to
				// remove. Don't loop around to read
				// the directory again. We'll probably
				// just get the same error.
				return logit(err)
			}
		}
	}

	// Remove directory.
	err1 := u.Remove(path)
	if err1 == nil || os.IsNotExist(err1) {
		return logit(nil)
	}

	if err == nil {
		err = err1
	}

	return logit(err)
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
		return logit(err)
	}

	if err := u.CanWriteInode(newpath); err != nil {
		return logit(err)
	}

	return os.Rename(oldpath, newpath)
}

func (u *user) Symlink(oldname, newname string) error {
	stat, _, err := u.hasInodeAccess(newname, Write)
	if err != nil {
		return logit(err)
	}

	if err = os.Symlink(oldname, newname); err != nil {
		return logit(err)
	}

	return u.chownNewFile(newname, u.gidForNewFiles(stat))
}

func (u *user) Truncate(name string, size int64) error {
	if err := u.CanWriteObject(name); err != nil {
		return logit(err)
	}

	return os.Truncate(name, size)
}

func (u *user) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := u.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return logit(err)
	}

	_, err = f.Write(data)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}

	return logit(err)
}

func (u *user) Stat(name string) (os.FileInfo, error) {
	if err := u.CanReadInode(name); err != nil {
		return nil, logit(err)
	}

	s, err := os.Stat(name)

	return s, logit(err)
}

func (u *user) Lstat(name string) (os.FileInfo, error) {
	if err := u.CanReadInode(name); err != nil {
		return nil, logit(err)
	}

	s, err := os.Lstat(name)

	return s, logit(err)
}

type file struct {
	*os.File
	u *user
}

func (f *file) Chdir() error {
	if err := f.u.checkDirExecuteOnly(f.Name()); err != nil {
		return logit(err)
	}

	return logit(f.File.Chdir())
}

func (f *file) Chmod(mode os.FileMode) error {
	return logit(f.u.Chmod(f.Name(), mode))
}

func (f *file) Chown(uid, gid int) error {
	return logit(f.u.Chown(f.Name(), uid, gid))
}

func (f *file) Readdir(n int) ([]os.FileInfo, error) {
	if err := f.u.checkDirExecuteOnly(f.Name()); err != nil {
		return nil, logit(err)
	}

	l, err := f.File.Readdir(n)

	return l, logit(err)
}

// Create checks for permissions and calls os.Create.
func (u *user) Create(name string) (File, error) {
	stat, _, err := u.hasInodeAccess(name, Write)
	if err != nil {
		return nil, logit(err)
	}

	f, err := os.Create(name)
	if err != nil {
		return nil, logit(err)
	}

	err = u.chownNewFile(name, u.gidForNewFiles(stat))
	if err != nil {
		f.Close()
		os.Remove(name) //nolint:errcheck

		return nil, logit(err)
	}

	return &file{f, u}, logit(nil)
}

func (u *user) Open(name string) (File, error) {
	if err := u.CanReadObject(name); err != nil {
		return nil, logit(err)
	}

	f, err := os.Open(name)
	if err != nil {
		return nil, logit(err)
	}

	return &file{f, u}, logit(nil)
}

func (u *user) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	stat, a, err := u.hasInodeAccess(name, Execute)
	if err != nil {
		return nil, logit(err)
	}

	var f *os.File

	for {
		// Try to open file exclusively first
		f, err = os.OpenFile(name, flag, perm|fs.FileMode(os.O_EXCL))

		// If the file exists, proceed to open an existing file, but only if O_EXCL wasn't passed as option
		if os.IsExist(err) && perm&fs.FileMode(os.O_EXCL) == 0 {
			p := Write

			if perm&fs.FileMode(os.O_RDONLY) > 0 {
				p = Read
			}

			// Check permission for accessing the existing file
			err := u.hasObjectAccess(name, p)
			if os.IsNotExist(err) {
				// The file disappeared in between OpenFile and Stat
				// Retry exclusive OpenFile, it makes no sense to return
				// file not found since O_EXCL was not passed as option.
				continue
			} else if err != nil {
				return nil, logit(err)
			}

			f, err := os.OpenFile(name, flag, perm)
			if err != nil {
				return nil, logit(err)
			}

			return &file{f, u}, nil
		} else if err != nil {
			return nil, logit(err)
		}

		break
	}

	// Check permission for creating a new file
	if err = u.checkPermission(stat, a, Write); err != nil && u.UID > 0 {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, logit(err)
	}

	if err = u.chownNewFile(name, u.gidForNewFiles(stat)); err != nil {
		os.Remove(name) //nolint:errcheck
		f.Close()

		return nil, logit(err)
	}

	return &file{f, u}, logit(nil)
}

func (u *user) ReadDir(name string) ([]os.DirEntry, error) {
	f, err := u.Open(name)
	if err != nil {
		return nil, logit(err)
	}

	defer f.Close()

	dirs, err := f.ReadDir(-1)
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name() < dirs[j].Name() })

	return dirs, logit(err)
}

func (u *user) EvalSymlinks(name string) (string, error) {
	name = filepath.Clean(name)

	// Resolve symlinks
	intermediate, err := ResolveSymlinks(name)
	if err != nil {
		return "", logit(err)
	}

	if len(intermediate) == 0 {
		return "", logit(os.ErrInvalid)
	}

	// Check whether directories are traversable
	for _, dir := range intermediate[:len(intermediate)-1] {
		if err := u.checkDirExecuteOnly(dir); err != nil {
			return "", logit(err)
		}
	}

	return intermediate[len(intermediate)-1], logit(nil)
}

func (u *user) Walk(root string, fn filepath.WalkFunc) error {
	info, err := u.Lstat(root)
	if err != nil {
		err = fn(root, nil, err)
	} else {
		err = u.walk(root, info, fn)
	}

	if err == filepath.SkipDir || err == filepath.SkipAll {
		return logit(nil)
	}

	return logit(err)
}

// walk recursively descends path, calling walkFn.
func (u *user) walk(path string, info fs.FileInfo, walkFn filepath.WalkFunc) error {
	if !info.IsDir() {
		return logit(walkFn(path, info, nil))
	}

	names, err := u.readDirNames(path)
	err1 := walkFn(path, info, err)
	// If err != nil, walk can't walk into this directory.
	// err1 != nil means walkFn want walk to skip this directory or stop walking.
	// Therefore, if one of err and err1 isn't nil, walk will return.
	if err != nil || err1 != nil {
		// The caller's behavior is controlled by the return value, which is decided
		// by walkFn. walkFn may ignore err and return nil.
		// If walkFn returns SkipDir or SkipAll, it will be handled by the caller.
		// So walk should return whatever walkFn returns.
		return logit(err)
	}

	for _, name := range names {
		filename := filepath.Join(path, name)

		fileInfo, err := u.Lstat(filename)
		if err != nil {
			if err := walkFn(filename, fileInfo, err); err != nil && err != filepath.SkipDir {
				return logit(err)
			}
		} else {
			err = u.walk(filename, fileInfo, walkFn)
			if err != nil {
				if !fileInfo.IsDir() || err != filepath.SkipDir {
					return logit(err)
				}
			}
		}
	}

	return logit(nil)
}

// readDirNames reads the directory named by dirname and returns
// a sorted list of directory entry names.
func (u *user) readDirNames(dirname string) ([]string, error) {
	f, err := u.Open(dirname)
	if err != nil {
		return nil, logit(err)
	}

	names, err := f.Readdirnames(-1)

	f.Close()

	if err != nil {
		return nil, logit(err)
	}

	sort.Strings(names)

	return names, logit(nil)
}
