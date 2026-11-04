package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var nativeMCPConfigPath string

// MCP management deliberately uses the same TOML file as Wardrobe.
func SetMCPConfigPath(path string) error {
	if path == "" || strings.ContainsAny(path, "\r\n\x00") {
		return errors.New("invalid TOML configuration path")
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		nativeMCPConfigPath = abs
	}
	return err
}
func mcpConfigPath() string {
	if nativeMCPConfigPath != "" {
		return nativeMCPConfigPath
	}
	if v := os.Getenv("WARDROBE_CONFIG"); v != "" {
		abs, _ := filepath.Abs(v)
		return abs
	}
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "Data", "config.toml")
}
func mcpConfigArguments() []string { return []string{"--config", mcpConfigPath()} }
func ConfigDir() (string, error) {
	// Keep Tunnel credentials separate from user SQLite and photos.
	p := filepath.Join(filepath.Dir(mcpConfigPath()), "..", "Private")
	return filepath.Abs(p)
}
func mcpStdioCommand(executable, cfg string) string {
	quote := func(v string) string { return strconv.Quote(strings.ReplaceAll(v, "\\", "/")) }
	return quote(executable) + " --mcp-stdio --config " + quote(cfg)
}
func validateMCPConfig() error {
	path := mcpConfigPath()
	if strings.ContainsAny(path, "\r\n\x00") {
		return errors.New("invalid TOML configuration path")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return errors.New("create Wardrobe Data/config.toml before saving Tunnel configuration")
	}
	_, err = loadConfig(path, true)
	return err
}
