package useros

import "errors"

var ErrTypeAssertion = errors.New("type assertion")

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
