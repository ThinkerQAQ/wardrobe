//go:build !windows

package main

import (
	"os"
	"syscall"
)

func mcpTryConfigurationLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func mcpUnlockConfiguration(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
