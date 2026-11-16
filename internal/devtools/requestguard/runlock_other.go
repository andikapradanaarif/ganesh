//go:build !darwin && !linux

package requestguard

import "os"

const runNoFollow = 0

func runLock(*os.File) error         { return ErrStopped }
func runUnlock(*os.File)             {}
func runSingleLink(os.FileInfo) bool { return false }
