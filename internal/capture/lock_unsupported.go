//go:build !darwin && !linux

package capture

import (
	"context"
	"errors"
)

func lock(context.Context, string) (func(), error) {
	return nil, errors.New("local spool platform unsupported")
}
