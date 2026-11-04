//go:build linux

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func mcpAutoManage(action string, executable string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	unit := filepath.Join(home, ".config", "systemd", "user", "wardrobe-tunnel.service")
	enabled := exec.Command("systemctl", "--user", "is-enabled", "wardrobe-tunnel.service").Run() == nil
	if data, e := os.ReadFile(unit); e == nil && !bytes.Contains(data, []byte(executable)) {
		return false, errors.New("another installation owns Tunnel startup")
	}
	switch action {
	case "status":
		return enabled, nil
	case "enable":
		if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\r\n") {
			return false, errors.New("invalid executable path")
		}
		if err = os.MkdirAll(filepath.Dir(unit), 0700); err != nil {
			return false, err
		}
		// ExecStart is tokenized by systemd, so quote and escape the path.
		escaped := strings.ReplaceAll(strings.ReplaceAll(executable, "\\", "\\\\"), "\"", "\\\"")
		content := "[Unit]\nDescription=Wardrobe user Tunnel\nAfter=network-online.target\n[Service]\nType=simple\nExecStart=\"" + escaped + "\" tunnel-run --config " + systemdQuote(mcpConfigPath()) + "\nRestart=on-failure\nRestartSec=5\nUMask=0077\n[Install]\nWantedBy=default.target\n"
		if err = privateWrite(unit, []byte(content)); err != nil {
			return false, err
		}
		if err = exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
			return false, err
		}
		if err = exec.Command("systemctl", "--user", "enable", "wardrobe-tunnel.service").Run(); err != nil {
			return false, err
		}
		return true, nil
	case "disable":
		if !enabled {
			return false, nil
		}
		if err = exec.Command("systemctl", "--user", "disable", "wardrobe-tunnel.service").Run(); err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, errors.New("unsupported auto-start action")
	}
}

func systemdQuote(s string) string {
	return "\"" + strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\""), "%", "%%") + "\""
}
