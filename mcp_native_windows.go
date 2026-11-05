//go:build windows

package main

import (
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
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

// Windows user logon startup uses the registry Run key to launch the
// native process directly. It does not spawn PowerShell, Bash or CMD.
const (
	mcpRunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	mcpRunValue   = "Wardrobe-Tunnel"
)

func mcpRunCommand(executable string) string {
	if strings.EqualFold(filepath.Base(executable), "wardrobe.exe") {
		host := filepath.Join(filepath.Dir(executable), "wardrobe-host.exe")
		if _, err := os.Stat(host); err == nil {
			executable = host // Native GUI subsystem: no console at login.
		}
	}
	return "\"" + filepath.Clean(executable) + "\" tunnel-run"
}

func mcpExistingRunValue(want string) (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, mcpRunKeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer k.Close()
	actual, _, err := k.GetStringValue(mcpRunValue)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(strings.TrimSpace(actual), want) {
		return false, errors.New("another installation owns the login startup entry")
	}
	return true, nil
}

// Inspect the old Task Scheduler entry before replacing it. This is used
// only for migrating or removing the previous PowerShell-wrapped task.
func mcpLegacyScheduledTask(executable string) (present, enabled bool, err error) {
	cmd := exec.Command("schtasks.exe", "/query", "/tn", mcpRunValue, "/xml")
	mcpHideProcess(cmd)
	data, runErr := cmd.CombinedOutput()
	data = mcpTaskXMLUTF8(data)
	if runErr != nil {
		message := strings.ToLower(strings.TrimSpace(string(data)))
		if strings.Contains(message, "cannot find") ||
			strings.Contains(message, "does not exist") ||
			strings.Contains(message, "cannot be found") ||
			strings.Contains(message, "找不到") ||
			strings.Contains(message, "不存在") {
			return false, false, nil
		}
		return false, false, fmt.Errorf("query legacy startup task: %w: %s", runErr, message)
	}
	var task struct {
		Actions struct {
			Exec []struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
		Settings struct {
			Enabled string `xml:"Enabled"`
		} `xml:"Settings"`
	}
	if err := xml.Unmarshal(data, &task); err != nil {
		return false, false, fmt.Errorf("decode legacy task: %w", err)
	}
	if len(task.Actions.Exec) != 1 {
		return false, false, errors.New("another installation owns the legacy startup task")
	}
	action := task.Actions.Exec[0]
	direct := strings.EqualFold(filepath.Clean(strings.Trim(action.Command, "\"")), filepath.Clean(executable)) &&
		strings.Contains(strings.ToLower(action.Arguments), "tunnel-run")
	legacy := strings.EqualFold(filepath.Base(action.Command), "powershell.exe") &&
		strings.Contains(strings.ToLower(action.Arguments), strings.ToLower(filepath.Clean(executable))) &&
		strings.Contains(strings.ToLower(action.Arguments), "tunnel-run")
	if !direct && !legacy {
		return false, false, errors.New("another installation owns the legacy startup task")
	}
	return true, !strings.EqualFold(strings.TrimSpace(task.Settings.Enabled), "false"), nil
}

func mcpTaskXMLUTF8(data []byte) []byte {
	if len(data) >= 2 && ((data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff)) {
		little := data[0] == 0xff
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			if little {
				units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
			} else {
				units = append(units, binary.BigEndian.Uint16(data[i:i+2]))
			}
		}
		data = []byte(string(utf16.Decode(units)))
	} else if len(data) >= 4 && data[1] == 0 && data[3] == 0 {
		units := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
		}
		data = []byte(string(utf16.Decode(units)))
	}
	return []byte(strings.Replace(string(data), "encoding=\"UTF-16\"", "encoding=\"UTF-8\"", 1))
}

func mcpRemoveLegacyTask() error {
	cmd := exec.Command("schtasks.exe", "/delete", "/tn", mcpRunValue, "/f")
	mcpHideProcess(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete owned legacy task: %w: %s", err, strings.TrimSpace(string(mcpTaskXMLUTF8(out))))
	}
	return nil
}

// mcpAutoManage never depends on a script interpreter. The legacy task is
// only removed after its action is confirmed to belong to this installation.
func mcpAutoManage(action string, executable string) (bool, error) {
	if action != "status" && action != "enable" && action != "disable" {
		return false, errors.New("invalid auto-start action")
	}
	command := mcpRunCommand(executable)
	present, err := mcpExistingRunValue(command)
	if err != nil {
		return false, err
	}
	legacy, legacyEnabled, err := mcpLegacyScheduledTask(executable)
	if err != nil {
		return false, err
	}
	if action == "status" {
		return present || legacyEnabled, nil
	}
	if action == "enable" {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, mcpRunKeyPath, registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			return false, err
		}
		err = k.SetStringValue(mcpRunValue, command)
		k.Close()
		if err != nil {
			return false, err
		}
		if legacy {
			if err := mcpRemoveLegacyTask(); err != nil {
				return true, err
			}
		}
		return true, nil
	}
	if present {
		k, err := registry.OpenKey(registry.CURRENT_USER, mcpRunKeyPath, registry.SET_VALUE)
		if err != nil {
			return false, err
		}
		err = k.DeleteValue(mcpRunValue)
		k.Close()
		if err != nil {
			return false, err
		}
	}
	if legacy {
		if err := mcpRemoveLegacyTask(); err != nil {
			return false, err
		}
	}
	return false, nil
}
func mcpExeBase(executable string) string { return filepath.Base(executable) }

var _ = os.ErrClosed
