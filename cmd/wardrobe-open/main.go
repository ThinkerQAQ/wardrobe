//go:build darwin

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	current, err := os.Executable()
	if err != nil {
		fail(err)
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "../../.."))
	cmd := exec.Command(filepath.Join(root, "wardrobe"), "-open=true")
	cmd.Dir = root
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		fail(err)
	}
	_ = cmd.Process.Release()
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
