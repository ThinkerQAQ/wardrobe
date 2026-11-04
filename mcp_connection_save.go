package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type mcpConnectionAction struct {
	Action           string `json:"action"`
	TunnelID         string `json:"tunnel_id"`
	RuntimeAPIKey    string `json:"runtime_api_key"`
	Replace          bool   `json:"replace"`
	PreviousTunnelID string `json:"previous_tunnel_id"`
	ReuseKey         bool   `json:"reuse_key"`
}

var mcpSaveMu sync.Mutex

func (a *application) handleMCPSave(w http.ResponseWriter, r *http.Request, in mcpConnectionAction) {
	if !validNativeTunnel(in.TunnelID) {
		http.Error(w, "invalid or fixture Tunnel ID", http.StatusBadRequest)
		return
	}
	if !in.ReuseKey && !validRuntimeKey(in.RuntimeAPIKey) {
		http.Error(w, "invalid Runtime API Key", http.StatusBadRequest)
		return
	}
	if !mcpSaveMu.TryLock() {
		http.Error(w, "a save is already running", 409)
		return
	}
	p, err := localMCPPaths()
	if err != nil {
		mcpSaveMu.Unlock()
		http.Error(w, "private config unavailable", 500)
		return
	}
	writeMCPState(p.State, "running")
	go func() {
		defer mcpSaveMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 95*time.Second)
		defer cancel()
		if err := nativeSave(ctx, in); err != nil {
			writeMCPState(p.State, "failed")
			return
		}
		writeMCPState(p.State, "succeeded")
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
}
