package main

import (
	"context"
	"errors"
	"os"
)

// ManageLocalMCP exposes the same native lifecycle to CLI and authenticated GUI.
// No management operation is published as a remote ChatGPT MCP tool.
func ManageLocalMCP(action string) (any, error) {
	switch action {
	case "status":
		return (&application{}).MCPConnection(context.Background())
	case "start", "stop", "restart":
		return nil, nativeAction(action)
	case "connect", "disconnect":
		return nil, mcpControlTunnel(action)
	case "auto-status", "auto-enable", "auto-disable":
		exe, err := os.Executable()
		if err != nil {
			return nil, err
		}
		enabled, err := mcpAutoManage(action[5:], exe)
		return map[string]bool{"auto_start": enabled}, err
	case "auto-connect-enable", "auto-connect-disable":
		return nil, mcpSetAutoConnect(action == "auto-connect-enable")
	default:
		return nil, errors.New("unknown local MCP action")
	}
}

// ConfigureLocalMCP explicitly configures a new connection in NativeMCP.
// It never imports the obsolete script-based installation.
func ConfigureLocalMCP(ctx context.Context, id, key string, reuseKey bool) error {
	p, err := localMCPPaths()
	if err != nil {
		return err
	}
	old := readTunnelID(p.Profile)
	return nativeSave(ctx, mcpConnectionAction{TunnelID: id, RuntimeAPIKey: key, ReuseKey: reuseKey, Replace: old != "", PreviousTunnelID: old})
}
