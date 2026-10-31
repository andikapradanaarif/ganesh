//go:build darwin || linux

package capture

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

func openNoFollow(path string, flags int) (*os.File, error) {
	return os.OpenFile(path, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
}

func lock(ctx context.Context, path string) (func(), error) {
	f, err := openNoFollow(path, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return nil, ErrUnsafe
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, ErrStorage
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		_ = f.Close()
		return nil, ErrUnsafe
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = f.Close()
			return nil, ErrStorage
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}
