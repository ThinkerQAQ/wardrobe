//go:build !windows

package main

import "os/exec"

func configureMCPProcess(cmd *exec.Cmd, setup bool) {}
