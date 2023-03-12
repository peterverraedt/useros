package useros

import "os"

type Permission uint32

const (
	Read    Permission = 4
	Write   Permission = 2
	Execute Permission = 1
)

func (p Permission) Check(m os.FileMode) bool {
	return uint32(p)&uint32(m) > 0
}
