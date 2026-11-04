//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// Only explicit first-time setup gets a visible input window. Polls and
// lifecycle actions remain silent when launched by the background application.
func configureMCPProcess(cmd *exec.Cmd, setup bool) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	if setup {
		cmd.SysProcAttr.CreationFlags = 0x00000010 // CREATE_NEW_CONSOLE
	} else {
		cmd.SysProcAttr.HideWindow = true
		cmd.SysProcAttr.CreationFlags = 0x08000000 // CREATE_NO_WINDOW
	}
}
