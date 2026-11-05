//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var shell32 = syscall.NewLazyDLL("shell32.dll")
var shellExecute = shell32.NewProc("ShellExecuteW")
var user32 = syscall.NewLazyDLL("user32.dll")
var messageBox = user32.NewProc("MessageBoxW")

func nativeShellOpen(target string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	item, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	r, _, callErr := shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(item)), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW error %d: %v", r, callErr)
	}
	return nil
}

func notifyGUIError(err error) {
	if err == nil {
		return
	}
	title, _ := syscall.UTF16PtrFromString("Wardrobe")
	msg, _ := syscall.UTF16PtrFromString(err.Error())
	messageBox.Call(0, uintptr(unsafe.Pointer(msg)), uintptr(unsafe.Pointer(title)), 0x10)
}

func openNativeGUI(mode string) error {
	host, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(host)
	if mode == "--guide-extension" {
		extension := filepath.Join(root, "browser-extension")
		if _, err := os.Stat(extension); err != nil {
			return err
		}
		if err := nativeShellOpen(extension); err != nil {
			return err
		}
		guide := filepath.Join(root, "README_ZH.md")
		if _, err := os.Stat(guide); err == nil {
			return nativeShellOpen(guide)
		}
		return nil
	}
	if mode != "--open-ui" {
		return errors.New("unsupported GUI action")
	}
	executable := filepath.Join(root, "wardrobe.exe")
	if _, err := os.Stat(executable); err != nil {
		return err
	}
	cfgPath := filepath.Join(root, "Data", "config.toml")
	if err := ensureDefaultConfig(cfgPath); err != nil {
		return err
	}
	cfg, err := loadConfig(cfgPath, false)
	if err != nil {
		return err
	}
	if err := validateAddress(cfg.Server.Address); err != nil {
		return err
	}
	url := "http://" + cfg.Server.Address
	if !existingWardrobe(url) {
		cmd := exec.Command(executable, "-open=false")
		cmd.Dir = root
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, HideWindow: true}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			return err
		}
		_ = cmd.Process.Release()
		deadline := time.Now().Add(12 * time.Second)
		for !existingWardrobe(url) && time.Now().Before(deadline) {
			time.Sleep(250 * time.Millisecond)
		}
		if !existingWardrobe(url) {
			return errors.New("local Wardrobe service failed health check")
		}
	}
	return nativeShellOpen(url)
}
