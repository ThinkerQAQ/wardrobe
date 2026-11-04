package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// A cross-process lock serializes Save with startup snapshot reads, including
// independently started Bridge, CLI, and login-service processes.
func mcpConfigurationLock(private string) (func(), error) {
	if err := os.MkdirAll(private, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(private, "configuration.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err = mcpTryConfigurationLock(f); err == nil {
			return func() { _ = mcpUnlockConfiguration(f); _ = f.Close() }, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			return nil, errors.New("another Tunnel configuration operation is in progress")
		}
		time.Sleep(30 * time.Millisecond)
	}
}
