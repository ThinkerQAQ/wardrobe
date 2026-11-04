package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeMCPStaysOptionalAndNeverLeaksKeys(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	previous := nativeMCPConfigPath
	t.Cleanup(func() { nativeMCPConfigPath = previous })
	config := filepath.Join(root, "Data", "config.toml")
	if err := SetMCPConfigPath(config); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultConfig(config); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultConfig(config); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "127.0.0.1:17936") {
		t.Fatal("default local config missing")
	}
	status, err := (&application{}).MCPConnection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Ready {
		t.Fatal("Tunnel Ready without verified health")
	}
	if status.RuntimeKeySaved {
		t.Fatal("secret unexpectedly present")
	}
	blob, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(blob), "runtime_api_key") {
		t.Fatal("status must not expose secret")
	}
	if !strings.Contains(string(blob), "mcpServers") {
		t.Fatal("offline STDIO must remain available")
	}
	original := string(raw)
	if err := ensureDefaultConfig(config); err != nil {
		t.Fatal(err)
	}
	persisted, _ := os.ReadFile(config)
	if string(persisted) != original {
		t.Fatal("first run must not overwrite existing user TOML")
	}
}

func TestNativeMCPHTTPDeniesCrossOrigin(t *testing.T) {
	store, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.db.Close()
	srv := &application{host: "127.0.0.1:17936", s: store}
	for _, origin := range []string{"https://example.com", "chrome-extension://abcdefghijklmnopabcdefghijklmnop"} {
		r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:17936/api/mcp-connection", strings.NewReader(`{"action":"stop"}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("origin %s: want 403, got %d", origin, w.Code)
		}
	}
}
