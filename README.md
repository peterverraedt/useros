Wrapper for `pkg.go.dev/os` that checks file permissions for a given user, when the process is executing as root, avoiding a setuid call.

Only linux is supported, on other OSes it is a simple wrapper around the `pkg.go.dev/os` package without further restrictions. If not run as the root user, it also simply wrapps `pkg.go.dev/os`. We enforce the standard permission model (user, group, other) as well as ACL lists. Since we still act as root, it works well with standard linux mounts, but will fail for e.g. kerberized nfs mounts if the root user is not mapped to the remote root.

Since we need to check file permissions before doing the actual file operation, we cannot garantee that the resulting operation is atomic.

Use at own risk. Usage is fairly simple:

```golang
useros := User{UID: 1000, GID: 1000, Groups: []int{1001}}.OS()

f, err := useros.Open("/path/to/file/name")
```

It implements the following interface:

```golang
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
```

Note: to run the golang tests, execute as root.
