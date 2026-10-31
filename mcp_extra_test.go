package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func lookbookFixture(t *testing.T, a *application) string {
	t.Helper()
	if e := os.MkdirAll(filepath.Join(a.s.dir, "images"), 0700); e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"W0001", "W0002", "W0003", "W0004"} {
		i := image.NewRGBA(image.Rect(0, 0, 320, 480))
		for y := 0; y < 480; y++ {
			for x := 0; x < 320; x++ {
				i.Set(x, y, color.RGBA{uint8(x % 250), uint8((y + x) % 253), uint8((y * 3) % 251), 255})
			}
		}
		var buf bytes.Buffer
		if e := jpeg.Encode(&buf, i, &jpeg.Options{Quality: 90}); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(a.s.dir, "images", id+".jpg"), buf.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		p := record{"name": "测试商品 " + id, "color": "深灰", "size": "M", "image": "/media/images/" + id + ".jpg"}
		v := 0
		if current, e := a.s.get("products", id); e == nil {
			v = current["version"].(int)
		}
		if _, e := a.s.save("products", id, p, v); e != nil {
			t.Fatal(e)
		}
	}
	tryonPhotoFixture(t, a, "LOOK-REF")
	_, e := a.s.save("looks", "LOOK-CARD", record{"name": "测试穿搭", "slots": map[string]string{"outer": "W0001", "inner": "W0002", "pants": "W0003", "shoes": "W0004"}}, 0)
	if e != nil {
		t.Fatal(e)
	}
	return "LOOK-CARD"
}
func TestLookbookProducesRealPNGAndRejectsInvalidData(t *testing.T) {
	a := testApp(t)
	id := lookbookFixture(t, a)
	in := lookbookRenderInput{LookID: id, PortraitLookID: "LOOK-REF"}
	b, e := a.renderLookbookCard(in)
	if e != nil {
		t.Fatal(e)
	}
	cfg, e := png.DecodeConfig(bytes.NewReader(b))
	if e != nil || cfg.Width != 1200 || cfg.Height != 1800 {
		t.Fatal("bad PNG", cfg, e)
	}
	if r := request(a, "POST", "/api/lookbook/render", `{"lookId":"LOOK-CARD","portraitLookId":"LOOK-REF"}`); r.Code != 200 || r.Header().Get("Content-Type") != "image/png" {
		t.Fatal("HTTP preview failed", r.Code, r.Body.String())
	}
	bad := []lookbookRenderInput{
		{LookID: id}, {LookID: id, PortraitLookID: "LOOK-MISSING"},
		{LookID: id, PortraitLookID: "LOOK-REF", Slots: map[string]string{"outer": "../../etc/passwd"}},
		{LookID: id, PortraitLookID: "LOOK-REF", Slots: map[string]string{"outer": "NOT-OWNED"}},
		{LookID: id, PortraitLookID: "LOOK-REF", HairID: "NOT-IN-DB"},
		{LookID: id, PortraitLookID: "LOOK-REF", PortraitBase64: "AAAA"},
	}
	for n, v := range bad {
		if _, e := a.renderLookbookCard(v); e == nil {
			t.Errorf("unsafe input %d accepted", n)
		}
	}
	if r := request(a, "GET", "/api/lookbook/render", ""); r.Code != 405 {
		t.Fatal("GET renderer must be rejected", r.Code)
	}
	if r := request(a, "POST", "/api/lookbook/render", `{"lookId":"LOOK-CARD","portraitLookId":"LOOK-REF","unknown":true}`); r.Code != 400 {
		t.Fatal("unknown fields accepted", r.Code)
	}
	if _, e := os.Stat(filepath.Join(a.s.dir, "tryon_evidence")); !os.IsNotExist(e) {
		t.Fatal("render must not create evidence files")
	}
}
func TestMCPExtraToolsAndPrivacyBoundary(t *testing.T) {
	a := testApp(t)
	id := lookbookFixture(t, a)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	ss, e := a.newMCPServer().Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	cl := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := cl.Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	tools, e := cs.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	available := map[string]bool{}
	for _, x := range tools.Tools {
		available[x.Name] = true
	}
	for _, name := range []string{"compose_wardrobe_lookbook", "get_wardrobe_capabilities", "get_wardrobe_tryon_evidence", "confirm_wardrobe_tryon_photo", "list_wardrobe_workflows", "get_wardrobe_workflow_source", "update_wardrobe_workflow", "reset_wardrobe_workflow", "delete_wardrobe_look"} {
		if !available[name] {
			t.Fatal("missing MCP tool", name)
		}
	}
	call := func(name string, args any) *mcp.CallToolResult {
		t.Helper()
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if e != nil {
			t.Fatal(name, e)
		}
		return r
	}
	r := call("get_wardrobe_capabilities", map[string]any{})
	if r.IsError {
		t.Fatal(r)
	}
	raw, _ := json.Marshal(r.StructuredContent)
	if !strings.Contains(string(raw), "managedRendererAvailable") || !strings.Contains(string(raw), "false") {
		t.Fatal("unavailable renderer not exposed")
	}
	r = call("compose_wardrobe_lookbook", lookbookRenderInput{LookID: id, PortraitLookID: "LOOK-REF"})
	if r.IsError {
		t.Fatal("MCP compositor failed", r)
	}
	count := 0
	for _, x := range r.Content {
		if im, ok := x.(*mcp.ImageContent); ok {
			count++
			cfg, e := png.DecodeConfig(bytes.NewReader(im.Data))
			if e != nil || cfg.Width != 1200 || cfg.Height != 1800 {
				t.Fatal("corrupt MCP image")
			}
		}
	}
	if count != 1 {
		t.Fatal("expected one PNG image")
	}
	r = call("get_wardrobe_tryon_evidence", IDInput{ID: "LOOK-REF"})
	if r.IsError {
		t.Fatal(r)
	}
	r = call("confirm_wardrobe_tryon_photo", tryonEvidenceInput{LookID: "LOOK-REF", Role: "face", ConfirmRealPhoto: false, ConfirmFrontal: true})
	if !r.IsError {
		t.Fatal("MCP self-confirmed photo without user assertion")
	}
	r = call("confirm_wardrobe_tryon_photo", tryonEvidenceInput{LookID: "LOOK-REF", Role: "face", ConfirmRealPhoto: true, ConfirmFrontal: true})
	if r.IsError {
		t.Fatal(r)
	}
	p, e := a.s.get("tryon_evidence", "EVID-LOOK-REF")
	if e != nil || p["verificationMethod"] != "mcp-user-declaration-not-biometric" {
		t.Fatal("provenance not marked correctly")
	}
	workflow, e := a.s.getWorkflow("wardrobe-outfits")
	if e != nil {
		t.Fatal(e)
	}
	edited := workflow.Content + "\n\nMCP revision test is reversible.\n"
	r = call("update_wardrobe_workflow", workflowEditInput{Name: "wardrobe-outfits", Version: workflow.Version, Content: edited})
	if r.IsError {
		t.Fatal("workflow edit failed", r)
	}
	r = call("update_wardrobe_workflow", workflowEditInput{Name: "wardrobe-outfits", Version: workflow.Version, Content: edited})
	if !r.IsError {
		t.Fatal("stale workflow edit accepted")
	}
	r = call("reset_wardrobe_workflow", workflowResetInput{Name: "wardrobe-outfits", Version: 1})
	if r.IsError {
		t.Fatal("reset workflow failed", r)
	}
	r = call("delete_wardrobe_look", lookDeleteInput{ID: id, Version: 1})
	if r.IsError {
		t.Fatal("delete look failed", r)
	}
	r = call("delete_wardrobe_look", lookDeleteInput{ID: id, Version: 1})
	if !r.IsError {
		t.Fatal("stale delete accepted")
	}
}
