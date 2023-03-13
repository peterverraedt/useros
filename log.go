package useros

import (
	"fmt"
	"runtime"
)

// LogHandler is a configurable log handler
var LogHandler func(error)

func logit(err error) error {
	if LogHandler == nil {
		return err
	}

	_, file, line, _ := runtime.Caller(1)

	if err != nil {
		LogHandler(fmt.Errorf("ERR at %s %d: %w", file, line, err))
	}

	return err
}
