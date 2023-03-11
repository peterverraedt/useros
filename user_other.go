//go:build !linux
// +build !linux

package useros

func (u User) OS() OS {
	return &def{}
}
