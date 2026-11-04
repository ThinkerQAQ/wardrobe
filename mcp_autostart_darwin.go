//go:build darwin

package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func mcpAutoManage(action, executable string) (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	unit := filepath.Join(home, "Library", "LaunchAgents", "com.thinkerqaq.wardrobe.tunnel.plist")
	data, err := os.ReadFile(unit)
	exists := err == nil
	if exists && !bytes.Contains(data, []byte(escapePlist(executable))) {
		return false, errors.New("another installation owns Tunnel LaunchAgent")
	}
	uid := strconv.Itoa(os.Getuid())
	label := "gui/" + uid + "/com.thinkerqaq.wardrobe.tunnel"
	switch action {
	case "status":
		if !exists {
			return false, nil
		}
		out, checkErr := exec.Command("launchctl", "print-disabled", "gui/"+uid).Output()
		if checkErr != nil {
			return false, checkErr
		}
		return !strings.Contains(string(out), "\"com.thinkerqaq.wardrobe.tunnel\" => true"), nil
	case "enable":
		if !filepath.IsAbs(executable) || strings.ContainsAny(executable, "\r\n") {
			return false, errors.New("invalid executable path")
		}
		content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.thinkerqaq.wardrobe.tunnel</string>
<key>ProgramArguments</key><array><string>` + escapePlist(executable) + `</string><string>tunnel-run</string><string>--config</string><string>` + escapePlist(mcpConfigPath()) + `</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><false/>
</dict></plist>`
		if err = privateWrite(unit, []byte(content)); err != nil {
			return false, err
		}
		// Enable on next login without bootstrapping a worker now.
		if err = exec.Command("launchctl", "enable", label).Run(); err != nil {
			return false, err
		}
		return true, nil
	case "disable":
		if !exists {
			return false, nil
		}
		// Disable the next login startup; never bootout a running worker.
		if err = exec.Command("launchctl", "disable", label).Run(); err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, errors.New("unsupported auto-start action")
	}
}
func escapePlist(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}
