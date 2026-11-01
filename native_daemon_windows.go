//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func newWardrobeDaemonCommand(path string) *exec.Cmd {
	command := exec.Command(path, "--open=false")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, HideWindow: true}
	return command
}
