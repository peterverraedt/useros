//go:build !linux
// +build !linux

package useros

func (u User) os() OS {
	return &def{}
}
