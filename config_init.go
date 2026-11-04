package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const defaultTOML = "[server]\naddress = \"127.0.0.1:17936\"\nopen_browser = false\n\n[storage]\ndata_dir = \".\"\n"

// Installers include only example config; first run creates user config if missing.
func ensureDefaultConfig(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = f.WriteString(defaultTOML); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
