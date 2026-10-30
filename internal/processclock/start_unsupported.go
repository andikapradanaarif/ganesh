//go:build !darwin || !cgo

package processclock

import (
	"errors"
	"time"
)

// Other platforms fail closed until their creation-time implementation is
// measured. The run preparation CLI still works with CGO disabled.
func Start() (time.Time, error) {
	return time.Time{}, errors.New("capture process clock unsupported on this build")
}
