package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

func runMCPManagement(args []string) error {
	// Optional explicit configuration path; no secrets in arguments.
	if len(args) >= 2 && (args[0] == "--config" || args[0] == "-config") {
		if err := SetMCPConfigPath(args[1]); err != nil {
			return err
		}
		args = args[2:]
	}
	if len(args) != 1 {
		return errors.New("usage: wardrobe mcp status|configure|install-client|start|stop|restart|connect|disconnect|auto-status|auto-enable|auto-disable|auto-connect-enable|auto-connect-disable")
	}
	switch args[0] {
	case "configure":
		var in struct {
			TunnelID      string `json:"tunnel_id"`
			RuntimeAPIKey string `json:"runtime_api_key"`
			ReuseKey      bool   `json:"reuse_key"`
		}
		dec := json.NewDecoder(io.LimitReader(os.Stdin, 8193))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return fmt.Errorf("read MCP settings from stdin: %w", err)
		}
		if dec.Decode(new(any)) != io.EOF {
			return errors.New("unexpected extra JSON input")
		}
		if in.RuntimeAPIKey != "" && in.ReuseKey {
			return errors.New("cannot provide and reuse key simultaneously")
		}
		p, err := localMCPPaths()
		if err != nil {
			return err
		}
		old := readTunnelID(p.Profile)
		return nativeSave(context.Background(), mcpConnectionAction{TunnelID: in.TunnelID, RuntimeAPIKey: in.RuntimeAPIKey, ReuseKey: in.ReuseKey, Replace: old != "", PreviousTunnelID: old})
	case "install-client":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		_, err = installTunnelClient(context.Background(), tunnelReleaseHTTPClient(), runtime.GOOS, runtime.GOARCH, home)
		return err
	default:
		out, err := ManageLocalMCP(args[0])
		if err != nil {
			return err
		}
		if out != nil {
			return json.NewEncoder(os.Stdout).Encode(out)
		}
		return nil
	}
}
