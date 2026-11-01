//go:build !windows

package main

import "os/exec"

func newWardrobeDaemonCommand(path string) *exec.Cmd { return exec.Command(path, "--open=false") }
