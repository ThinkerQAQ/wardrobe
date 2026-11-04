package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type mcpConnectionPreference struct {
	AutoConnect bool `json:"auto_connect"`
}
type mcpRunnerState struct {
	Token      string `json:"token"`
	Connecting bool   `json:"connecting"`
	ActionID   string `json:"action_id,omitempty"`
}
type mcpRunnerControlRequest struct {
	Token  string `json:"token"`
	Action string `json:"action"`
	ID     string `json:"id"`
}

func mcpAutoConnectPath(p nativeMCPPaths) string {
	return filepath.Join(p.Private, "auto-connect.json")
}
func mcpRunnerStatePath(p nativeMCPPaths) string {
	return filepath.Join(p.Private, "runner-state.json")
}
func mcpRunnerCommandPath(p nativeMCPPaths) string {
	return filepath.Join(p.Private, "runner-command.json")
}

// Existing Wardrobe runners always connect on Start. Missing prefs preserve
// that behavior; invalid prefs fail closed to prevent an unintended connection.
func mcpAutoConnectEnabled(p nativeMCPPaths) bool {
	b, err := os.ReadFile(mcpAutoConnectPath(p))
	if os.IsNotExist(err) {
		return true
	}
	if err != nil || len(b) > 1024 {
		return false
	}
	var pref mcpConnectionPreference
	if json.Unmarshal(b, &pref) != nil {
		return false
	}
	return pref.AutoConnect
}
func mcpSetAutoConnect(enabled bool) error {
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	release, err := mcpConfigurationLock(p.Private)
	if err != nil {
		return err
	}
	defer release()
	if readTunnelID(p.Profile) == "" {
		return errors.New("save Tunnel connection before editing preferences")
	}
	if _, err := mcpReadKey(p); err != nil {
		return errors.New("stored Tunnel Runtime API Key unavailable")
	}
	b, _ := json.Marshal(mcpConnectionPreference{AutoConnect: enabled})
	if err := privateWrite(mcpAutoConnectPath(p), b); err != nil {
		return err
	}
	if mcpRunnerRunning(p) {
		return privateWrite(p.Restart, []byte("true"))
	}
	return nil
}
func mcpReadRunnerState(p nativeMCPPaths) (mcpRunnerState, bool) {
	var state mcpRunnerState
	if !mcpRunnerRunning(p) {
		return state, false
	}
	info := readRunnerInfo(p)
	b, err := os.ReadFile(mcpRunnerStatePath(p))
	if err != nil || len(b) > 4096 {
		return state, false
	}
	if json.Unmarshal(b, &state) != nil || state.Token != info.Token {
		return mcpRunnerState{}, false
	}
	return state, true
}
func mcpWriteRunnerState(p nativeMCPPaths, state mcpRunnerState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return privateWrite(mcpRunnerStatePath(p), b)
}
func mcpControlTunnel(action string) error {
	if action != "connect" && action != "disconnect" {
		return errors.New("unsupported Tunnel action")
	}
	nativeLifecycleMu.Lock()
	defer nativeLifecycleMu.Unlock()
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	release, err := mcpConfigurationLock(p.Private)
	if err != nil {
		return err
	}
	defer release()
	if !mcpRunnerRunning(p) {
		return errors.New("start local Tunnel service before connecting")
	}
	if action == "connect" {
		if _, err := os.Stat(p.Restart); err == nil {
			return errors.New("restart service to apply saved changes before connecting")
		}
	}
	info := readRunnerInfo(p)
	id, err := nativeRevision()
	if err != nil {
		return err
	}
	b, _ := json.Marshal(mcpRunnerControlRequest{Token: info.Token, Action: action, ID: id})
	if err = privateWrite(mcpRunnerCommandPath(p), b); err != nil {
		return err
	}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if !mcpRunnerRunning(p) {
			return errors.New("Tunnel service stopped during command")
		}
		if state, ok := mcpReadRunnerState(p); ok && state.ActionID == id {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("Tunnel command was not acknowledged by local service")
}
