//go:build darwin || linux

package requestguard

import (
	"errors"
	"os"
	"syscall"
)

const runNoFollow = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

func runLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return ErrRunBusy
	}
	if err != nil {
		return ErrStopped
	}
	return nil
}
func runUnlock(f *os.File)             { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
func runSingleLink(f os.FileInfo) bool { s, ok := f.Sys().(*syscall.Stat_t); return ok && s.Nlink == 1 }
