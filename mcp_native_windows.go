//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func mcpProtectKey(clear []byte) ([]byte, error) {
	if len(clear) == 0 {
		return nil, errors.New("empty Runtime API Key")
	}
	in := windows.DataBlob{Size: uint32(len(clear)), Data: &clear[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("DPAPI key encryption: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func mcpUnprotectKey(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, errors.New("empty encrypted Runtime API Key")
	}
	in := windows.DataBlob{Size: uint32(len(cipher)), Data: &cipher[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, fmt.Errorf("DPAPI key decryption: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	return append([]byte(nil), unsafe.Slice(out.Data, int(out.Size))...), nil
}
func mcpHideProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
func mcpDetachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000200}
}
func mcpRunnerExclusive(_ string) (func(), error) {
	name, _ := windows.UTF16PtrFromString("Local\\ThinkerQAQ.Wardrobe.Tunnel.Runner")
	h, err := windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, errors.New("Tunnel runner is already active")
	}
	if err != nil {
		return nil, err
	}
	return func() { windows.CloseHandle(h) }, nil
}
func mcpProcessStartTime(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, errors.New("invalid process ID")
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return 0, err
	}
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), nil
}

// Windows changes the reported image path after an installer replaces a
// running EXE (e.g. *.tmpUpdate.*). PID + recorded creation time identifies
// the same process across updates without trusting that mutable path.
func mcpSameProcess(pid int, wantedExe string, startedAt uint64) bool {
	if pid <= 0 || wantedExe == "" || startedAt == 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != 259 { // STILL_ACTIVE
		return false
	}
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(h, &created, &exited, &kernel, &user) != nil {
		return false
	}
	current := uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime)
	return current == startedAt
}
func mcpAutoManage(action string, executable string) (bool, error) {
	const task = "Wardrobe-Tunnel"
	runner := "\"" + executable + "\" tunnel-run --config \"" + mcpConfigPath() + "\""
	// The scheduled task belongs to the current user, never SYSTEM.
	lookup := exec.Command("schtasks.exe", "/Query", "/TN", task, "/XML")
	mcpHideProcess(lookup)
	output, lookupErr := lookup.Output()
	exists := lookupErr == nil
	owned := exists && strings.Contains(strings.ToLower(string(output)), strings.ToLower(filepath.Clean(executable)))
	if exists && !owned {
		return false, errors.New("another installation owns the Tunnel task")
	}
	switch action {
	case "status":
		return exists, nil
	case "enable":
		cmd := exec.Command("schtasks.exe", "/Create", "/F", "/TN", task, "/SC", "ONLOGON", "/TR", runner, "/RL", "LIMITED")
		mcpHideProcess(cmd)
		if err := cmd.Run(); err != nil {
			return false, errors.New("unable to enable Tunnel at login")
		}
		return true, nil
	case "disable":
		if !exists {
			return false, nil
		}
		cmd := exec.Command("schtasks.exe", "/Delete", "/F", "/TN", task)
		mcpHideProcess(cmd)
		if err := cmd.Run(); err != nil {
			return false, errors.New("unable to disable Tunnel at login")
		}
		return false, nil
	}
	return false, errors.New("invalid auto-start action")
}
func mcpExeBase(executable string) string { return filepath.Base(executable) }

var _ = os.ErrClosed
