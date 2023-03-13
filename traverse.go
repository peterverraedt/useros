package useros

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// TraversedDirectories returns the list of directories that is followed to get the the specified inode.
// We first clean up the path, i.e. .. is replaced. We also follow symlinks.
// E.g. /sbin/../bin/bash would return [/, /usr, /usr/bin] because /bin is symlink to /usr/bin.
// The last directory should be the real path of the directory of the inode.
// Note that the existence or type of the inode itself is not checked.
func TraversedDirectories(path string) ([]string, error) {
	return ResolveSymlinks(filepath.Dir(filepath.Clean(path)))
}

// ResolveSymlinks returns the list of paths that is followed to get the the specified path.
func ResolveSymlinks(path string) ([]string, error) {
	if len(path) == 0 {
		return nil, os.ErrInvalid
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if path[0] == '.' {
		return nil, os.ErrInvalid
	}

	p := strings.IndexRune(path, os.PathSeparator)
	curdir := path[:p+1]
	result := []string{curdir}

	nlinks := 0

	for {
		// Find next part of the url
		n := strings.IndexRune(path[p+1:], os.PathSeparator) + p + 1

		next := path[:n]

		if n < p+1 {
			// Arrived at the last part
			n = len(path)
			next = path
		}

		fi, err := os.Lstat(next)
		if err != nil {
			return nil, err
		}

		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			nlinks++
			if nlinks > 16 {
				return nil, os.ErrInvalid
			}

			// Follow symlink
			target, err := os.Readlink(next)
			if err != nil {
				return nil, err
			}

			if !os.IsPathSeparator(target[0]) {
				target = filepath.Join(curdir, target)
			}

			// Move to target
			path = filepath.Clean(target)
			p = strings.IndexRune(path, os.PathSeparator)
			curdir = path[:p+1]
			result = append(result, curdir)

			continue
		}

		if !fi.IsDir() && n < len(path) {
			return nil, syscall.ENOTDIR
		}

		// Resolved last path
		if n >= len(path) {
			result = append(result, next)

			return result, nil
		}

		// nextdir is a directory
		p = n
		curdir = next
		result = append(result, curdir)
	}
}
