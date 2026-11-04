package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Management stays local to the authenticated application, separate from remote MCP tools.
var tunnelIDPattern = regexp.MustCompile(`^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$`)
var profileTunnelID = regexp.MustCompile(`(?m)^\s*tunnel_id:\s*["']?(tunnel_[a-z0-9_]+)`)

type MCPConnectionStatus struct {
	State             string `json:"state"`
	Version           string `json:"version"`
	TunnelID          string `json:"tunnel_id"`
	ClientInstalled   bool   `json:"client_installed"`
	Configured        bool   `json:"configured"`
	Ready             bool   `json:"ready"`
	LocalConfig       any    `json:"local_config"`
	RuntimeKeySaved   bool   `json:"runtime_key_saved"`
	Running           bool   `json:"running"`
	AutoStart         bool   `json:"auto_start"`
	SetupState        string `json:"setup_state"`
	Revision          string `json:"revision"`
	RestartRequired   bool   `json:"restart_required"`
	TunnelAutoConnect bool   `json:"tunnel_auto_connect"`
	TunnelConnecting  bool   `json:"tunnel_connecting"`
}

func findTunnelClient(goos, home string) string {
	exe, _ := os.Executable()
	near := filepath.Join(filepath.Dir(exe), "bin", "tunnel-client")
	if goos == "windows" {
		near += ".exe"
	}
	if st, err := os.Stat(near); err == nil && !st.IsDir() {
		return near
	}
	if path, err := exec.LookPath("tunnel-client"); err == nil {
		return path
	}
	path := tunnelClientTarget(goos, home)
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return path
	}
	return ""
}
func mcpPaths(dataDir string) (string, string) {
	// Tunnel state belongs to Wardrobe's existing data directory on every OS.
	// Never read or migrate the obsolete script-managed Tunnel configuration.
	p := filepath.Join(dataDir, "mcp")
	return filepath.Join(p, "wardrobe.yaml"), filepath.Join(p, "health.url")
}
func tunnelReady(ctx context.Context, client, health string) bool {
	if client == "" {
		return false
	}
	if st, err := os.Stat(health); err != nil || st.IsDir() {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 1800*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "health", "--url-file", health, "--require-control-plane-poll", "--json")
	mcpHideProcess(cmd)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
}
func (a *application) MCPConnection(ctx context.Context) (MCPConnectionStatus, error) {
	out := MCPConnectionStatus{State: "unconfigured", Version: appVersion}
	home, err := os.UserHomeDir()
	if err != nil {
		return out, err
	}
	exe, err := os.Executable()
	if err != nil {
		return out, err
	}
	args := []string{"--mcp-stdio", "--config", mcpConfigPath()}
	out.LocalConfig = map[string]any{"mcpServers": map[string]any{"wardrobe": map[string]any{"command": exe, "args": args}}}
	privatePaths, err := localMCPPaths()
	if err != nil {
		return out, err
	}
	profile, health := privatePaths.Profile, privatePaths.Health
	client := findTunnelClient(runtime.GOOS, home)
	out.ClientInstalled = client != ""
	out.Running = mcpRunnerRunning(privatePaths)
	if exe, err := os.Executable(); err == nil {
		out.AutoStart, _ = mcpAutoManage("status", exe)
	}
	out.TunnelAutoConnect = mcpAutoConnectEnabled(privatePaths)
	if state, ok := mcpReadRunnerState(privatePaths); ok {
		out.TunnelConnecting = state.Connecting
	}
	if f, err := os.Open(profile); err == nil {
		data, readErr := io.ReadAll(io.LimitReader(f, 128<<10))
		f.Close()
		if readErr == nil {
			m := profileTunnelID.FindSubmatch(data)
			if len(m) == 2 && validNativeTunnel(string(m[1])) {
				out.Configured = true
				out.TunnelID = string(m[1])
			}
		}
	}
	private := filepath.Dir(health)
	keyName := "runtime.env"
	if runtime.GOOS == "windows" {
		keyName = "runtime-key.dpapi"
	}
	if info, err := os.Stat(filepath.Join(private, keyName)); err == nil && !info.IsDir() && info.Size() > 0 {
		if _, err := mcpReadKey(privatePaths); err == nil {
			out.RuntimeKeySaved = true
		}
	}
	if b, err := os.ReadFile(filepath.Join(private, "setup-state")); err == nil {
		out.SetupState = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile(filepath.Join(private, "configuration-revision")); err == nil {
		out.Revision = strings.TrimSpace(string(b))
	}
	if info, err := os.Stat(filepath.Join(private, "restart-required")); err == nil && !info.IsDir() {
		out.RestartRequired = true
	}
	if !out.ClientInstalled {
		out.State = "client_missing"
	} else if out.Configured {
		out.State = "stopped_or_unhealthy"
		if out.Running {
			if out.TunnelConnecting {
				out.State = "starting_or_unhealthy"
			} else {
				out.State = "standby"
			}
		}
		out.Ready = out.Running && out.TunnelConnecting && tunnelReady(ctx, client, health)
		if out.Ready {
			out.State = "ready"
		}
	}
	return out, nil
}

// MCP runtime and credentials are managed by native Go.
func (a *application) handleMCPConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		status, err := a.MCPConnection(r.Context())
		if err != nil {
			http.Error(w, "MCP status unavailable", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in mcpConnectionAction
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid request", 400)
		return
	}
	switch in.Action {
	case "save":
		a.handleMCPSave(w, r, in)
		return
	case "install-client":
		home, err := os.UserHomeDir()
		if err != nil {
			http.Error(w, "home unavailable", 500)
			return
		}
		if _, err = installTunnelClient(r.Context(), tunnelReleaseHTTPClient(), runtime.GOOS, runtime.GOARCH, home); err != nil {
			http.Error(w, "official Tunnel Client verification or installation failed", 502)
			return
		}
	case "connect", "disconnect":
		if err := mcpControlTunnel(in.Action); err != nil {
			http.Error(w, "Tunnel operation failed: "+err.Error(), http.StatusConflict)
			return
		}
	case "auto-connect-enable", "auto-connect-disable":
		if err := mcpSetAutoConnect(in.Action == "auto-connect-enable"); err != nil {
			http.Error(w, "Tunnel connection preference update failed", 409)
			return
		}
	case "start", "stop", "restart":
		if err := nativeAction(in.Action); err != nil {
			http.Error(w, "Tunnel operation failed: "+err.Error(), http.StatusConflict)
			return
		}
	case "auto-status", "auto-enable", "auto-disable":
		exe, err := os.Executable()
		if err != nil {
			http.Error(w, "local executable unavailable", 500)
			return
		}
		action := strings.TrimPrefix(in.Action, "auto-")
		enabled, err := mcpAutoManage(action, exe)
		if err != nil {
			http.Error(w, "Tunnel startup management failed: "+err.Error(), 409)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"accepted": true, "auto_start": enabled})
		return
	default:
		http.Error(w, "unsupported MCP action", 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}

func (a *application) registerMCPManagement(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/mcp-connection", a.handleMCPConnection)
	mux.HandleFunc("POST /api/mcp-connection", a.handleMCPConnection)
}
