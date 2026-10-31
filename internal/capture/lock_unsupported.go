//go:build !darwin && !linux

package capture

import (
	"context"
	"errors"
	"os"
)

var errPlatform = errors.New("local capture storage is unsupported on this platform")

func openNoFollow(string, int) (*os.File, error)   { return nil, errPlatform }
func lock(context.Context, string) (func(), error) { return nil, errPlatform }
