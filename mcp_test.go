package main

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMCPToolsEndToEnd(t *testing.T) {
	a := testApp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	ss, e := a.newMCPServer().Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := client.Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	tools, e := cs.ListTools(ctx, nil)
	if e != nil || len(tools.Tools) != 30 {
		t.Fatal(tools, e)
	}
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	for _, name := range []string{"search_wardrobe", "list_wardrobe_looks", "get_wardrobe_rules", "audit_wardrobe", "list_wardrobe_hairstyles"} {
		r := call(name, map[string]any{})
		if r.IsError {
			t.Fatal(name, r)
		}
	}
	// Legacy notes remain in SQLite and the aesthetics Skill reads them live.
	_, e = a.s.save("notes", "NOTE-TEST", record{"title": "衣橱审美来源", "text": "user_note_keep_in_sqlite"}, 0)
	if e != nil {
		t.Fatal(e)
	}
	aestheticWorkflow := call("get_wardrobe_workflow", WorkflowInput{Name: "wardrobe-aesthetics"})
	if aestheticWorkflow.IsError {
		t.Fatal("aesthetics workflow failed", aestheticWorkflow)
	}
	ab, _ := json.Marshal(aestheticWorkflow.StructuredContent)
	if !strings.Contains(string(ab), "user_note_keep_in_sqlite") {
		t.Fatal("aesthetics notes not loaded live from SQLite")
	}
	workflow := call("get_wardrobe_workflow", WorkflowInput{Name: "wardrobe-import"})
	if workflow.IsError || !strings.Contains(workflow.Content[0].(*mcp.TextContent).Text, "wardrobe-import") {
		t.Fatal("cannot load bundled workflow", workflow)
	}
	r := call("get_wardrobe_product", IDInput{ID: "W0001"})
	if r.IsError {
		t.Fatal(r)
	}
	r = call("save_wardrobe_look", LookInput{Name: "MCP test", Rating: 4, Slots: map[string]string{"inner": "W0001"}})
	if r.IsError {
		t.Fatal(r)
	}
	b, _ := json.Marshal(r.StructuredContent)
	var saved RecordOutput
	json.Unmarshal(b, &saved)
	id := saved.Record["id"].(string)
	r = call("get_wardrobe_look", IDInput{ID: id})
	if r.IsError {
		t.Fatal(r)
	}
	r = call("save_wardrobe_look", LookInput{ID: id, Version: 1, Name: "Updated", Rating: 4, Slots: map[string]string{"inner": "W0001"}})
	if r.IsError {
		t.Fatal(r)
	}
	r = call("save_wardrobe_look", LookInput{ID: id, Version: 1, Name: "Stale", Rating: 4, Slots: map[string]string{}})
	if !r.IsError {
		t.Fatal("stale MCP update accepted")
	}
	r = call("update_wardrobe_review", ReviewInput{ID: "W0001", Version: 1, Rating: 5})
	if r.IsError {
		t.Fatal(r)
	}
	p, _ := a.s.get("products", "W0001")
	if p["rating"] != float64(5) || p["ratingSource"] != "user" {
		t.Fatal(p)
	}
	r = call("get_wardrobe_image", ImageInput{ID: "../../secret", Kind: "products"})
	if !r.IsError {
		t.Fatal("arbitrary file accepted")
	}
	r = call("save_wardrobe_look", LookInput{Name: "Invalid", Rating: 0, Slots: map[string]string{"inner": "UNKNOWN"}})
	if !r.IsError {
		t.Fatal("unknown ID accepted")
	}
}
func TestMCPHTTPAuth(t *testing.T) {
	a := testApp(t)
	token, e := ensureMCPToken(a.s.dir)
	if e != nil {
		t.Fatal(e)
	}
	again, e := ensureMCPToken(a.s.dir)
	if e != nil || again != token {
		t.Fatal("token is not persistent")
	}
	handler := mcpHTTP(a.newMCPServer(), token)
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	for _, auth := range []string{"", "Bearer wrong", "Bearer " + token} {
		r := httptest.NewRequest("POST", "http://127.0.0.1:17936/mcp", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("Authorization", auth)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if auth == "Bearer "+token {
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "wardrobe") {
				t.Fatal(w.Code, w.Body.String())
			}
		} else if w.Code != 401 {
			t.Fatal("missing auth accepted")
		}
	}
}

func TestWardrobeImageCaptionDistinguishesProvenance(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want string
	}{
		{"products", "商品本地归档主图"},
		{"looks", "核验原始来源"},
		{"hairstyles", "不是已核实的用户真人脸照"},
	} {
		got := wardrobeImageCaption(tc.kind, "TEST-1")
		if !strings.Contains(got, tc.want) {
			t.Fatalf("caption for %s should contain %q, got %q", tc.kind, tc.want, got)
		}
	}
}
