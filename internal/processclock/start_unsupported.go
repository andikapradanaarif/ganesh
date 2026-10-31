//go:build !darwin || !cgo

package processclock

import (
	"errors"
	"time"
)

func Start() (time.Time, error) {
	return time.Time{}, errors.New("capture process clock is unsupported on this build")
}
