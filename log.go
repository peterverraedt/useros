package useros

import (
	"log"
	"runtime"
)

func logit(err error) error {
	_, file, line, _ := runtime.Caller(1)

	if err != nil {
		log.Printf("ERR %s %d: %s", file, line, err)
	}

	return err
}
