package useros

import (
	"os"
	"path/filepath"
	"strings"
)

// Directories returns the list of directories that is followed to get the the specified path.
// We first clean up the path, i.e. .. is replaced. We also follow symlinks.
// E.g. /sbin/../bin/bash would return [/, /usr, /usr/bin] because /bin is symlink to /usr/bin.
func Directories(path string) ([]string, error) {
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

		if n < p+1 {
			return result, nil
		}

		nextdir := path[:n]

		fi, err := os.Lstat(nextdir)
		if err != nil {
			return nil, err
		}

		if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
			nlinks++
			if nlinks > 16 {
				return nil, os.ErrInvalid
			}

			// Follow symlink
			target, err := os.Readlink(nextdir)
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

		// nextdir is a directory
		p = n
		curdir = nextdir
		result = append(result, curdir)
	}
}
