package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"
)

// The secure Tunnel and current ChatGPT use MCP 2026-07-28 discovery.
// An adapter that only starts on "initialize" silently drops discovery and
// produces a control-plane command response deadline despite local Ready.
func TestTunnelStdioModernDiscoveryAndLegacyFallback(t *testing.T) {
	app := testApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	inReader, inWriter := io.Pipe()
	outReader, outWriter := io.Pipe()
	defer inWriter.Close()
	defer outReader.Close()
	done := make(chan error, 1)
	go func() {
		done <- runReinitializableMCPStdio(ctx, app.newMCPServer, inReader, outWriter)
		outWriter.Close()
	}()
	reader := bufio.NewReader(outReader)
	modernMeta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"wardrobe-modern-test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`
	send := func(id int, method string, modern bool) {
		t.Helper()
		params := "{}"
		if modern {
			params = "{" + modernMeta + "}"
		}
		message := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q,"params":%s}`, id, method, params)
		if _, err := io.WriteString(inWriter, message+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	receive := func(id int) map[string]any {
		t.Helper()
		type result struct {
			body []byte
			err  error
		}
		ch := make(chan result, 1)
		go func() { b, e := reader.ReadBytes('\n'); ch <- result{b, e} }()
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("MCP stdio read: %v", r.err)
			}
			var reply map[string]any
			if err := json.Unmarshal(r.body, &reply); err != nil {
				t.Fatal(err)
			}
			if reply["id"] != float64(id) {
				t.Fatalf("response id: got %v want %d", reply["id"], id)
			}
			if e, ok := reply["error"]; ok {
				t.Fatalf("MCP request %d failed: %v", id, e)
			}
			return reply
		case <-time.After(4 * time.Second):
			t.Fatalf("MCP stdio request %d timed out", id)
			return nil
		}
	}
	for _, id := range []int{1, 3} {
		send(id, "server/discover", true)
		response := receive(id)
		result, ok := response["result"].(map[string]any)
		if !ok {
			t.Fatalf("discover missing result: %v", response)
		}
		versions, ok := result["supportedVersions"].([]any)
		if !ok {
			t.Fatalf("discover missing supported versions: %v", result)
		}
		supported := false
		for _, v := range versions {
			if v == "2026-07-28" {
				supported = true
			}
		}
		if !supported {
			t.Fatalf("modern protocol missing: %v", versions)
		}
		send(id+1, "tools/list", true)
		list := receive(id + 1)
		tools, ok := list["result"].(map[string]any)["tools"].([]any)
		if !ok || len(tools) != 30 {
			t.Fatalf("modern tools/list: got %d tools, valid=%v", len(tools), ok)
		}
	}
	// A legacy client may connect after a modern one on the same long-lived Tunnel.
	legacy := `{"jsonrpc":"2.0","id":5,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"legacy-client","version":"1"}}}`
	if _, err := io.WriteString(inWriter, legacy+"\n"); err != nil {
		t.Fatal(err)
	}
	initialized := receive(5)
	if initialized["result"].(map[string]any)["protocolVersion"] != "2025-06-18" {
		t.Fatalf("legacy initialization failed: %v", initialized["result"])
	}
	if _, err := io.WriteString(inWriter, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"); err != nil {
		t.Fatal(err)
	}
	send(6, "tools/list", false)
	list := receive(6)
	if len(list["result"].(map[string]any)["tools"].([]any)) != 30 {
		t.Fatal("legacy tools/list after modern discovery failed")
	}
	inWriter.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Tunnel stdio did not exit after EOF")
	}
}
