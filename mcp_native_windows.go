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

// mcpPSQuote protects dynamic paths when passing a script to PowerShell.
func mcpPSQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }

// mcpAutoManage changes only this application's opt-in user logon task. The
// runtime's on-demand Native Messaging launch path works independently.
func mcpAutoManage(action string, executable string) (bool, error) {
	const task = "Wardrobe-Tunnel"
	executable = filepath.Clean(executable)
	runner := "& " + mcpPSQuote(executable) + " tunnel-run; exit $LASTEXITCODE"
	runnerArgs := "-NoProfile -NonInteractive -WindowStyle Hidden -Command \"" + runner + "\""
	quotedTask := mcpPSQuote(task)
	// Check ownership before overwriting or removing an existing task. Accept
	// both the old schtasks-created direct action and the new hidden wrapper.
	script := "$ErrorActionPreference='Stop'; " +
		"$t=Get-ScheduledTask -TaskPath '\\' -TaskName " + quotedTask + " -ErrorAction SilentlyContinue; " +
		"if($null -ne $t) { $a=$t.Actions[0]; " +
		"$owned=($t.Actions.Count -eq 1) -and (" +
		"($a.Execute -ieq " + mcpPSQuote(executable) + ") -or " +
		"(([IO.Path]::GetFileName($a.Execute) -ieq 'powershell.exe') -and " +
		"($a.Arguments.IndexOf(" + mcpPSQuote(executable) + ",[StringComparison]::OrdinalIgnoreCase) -ge 0))" +
		"); if(-not $owned){throw 'another installation owns the Tunnel task'} }; "
	switch action {
	case "status":
		script += "if($null -ne $t -and $t.Settings.Enabled){'true'}else{'false'}"
	case "enable":
		script += "$user=[Security.Principal.WindowsIdentity]::GetCurrent().Name; " +
			"$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument " + mcpPSQuote(runnerArgs) + "; " +
			"$p=New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited; " +
			"$trigger=New-ScheduledTaskTrigger -AtLogOn -User $user; " +
			"$settings=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries; " +
			"Register-ScheduledTask -TaskName " + quotedTask + " -Action $a -Principal $p -Trigger $trigger -Settings $settings -Force -ErrorAction Stop | Out-Null; 'true'"
	case "disable":
		script += "if($null -ne $t){Unregister-ScheduledTask -TaskName " + quotedTask + " -Confirm:$false -ErrorAction Stop}; 'false'"
	default:
		return false, errors.New("invalid auto-start action")
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	mcpHideProcess(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Preserve the actual Windows diagnostic rather than masking Access denied.
		return false, fmt.Errorf("Tunnel login startup %s: %w: %s", action, err, strings.TrimSpace(string(output)))
	}
	return strings.EqualFold(strings.TrimSpace(string(output)), "true"), nil
}
func mcpExeBase(executable string) string { return filepath.Base(executable) }

var _ = os.ErrClosed
