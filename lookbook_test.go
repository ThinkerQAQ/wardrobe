package main

import (
	"strings"
	"testing"
)

func TestLookbookBundledResourcesAndStrictComposition(t *testing.T) {
	for _, name := range []string{"web/lookbook.html", "web/lookbook.css", "web/lookbook.js"} {
		b, err := assets.ReadFile(name)
		if err != nil || len(b) == 0 {
			t.Fatalf("bundled lookbook resource missing: %s (%v)", name, err)
		}
	}
	js, _ := assets.ReadFile("web/lookbook.js")
	s := string(js)
	for _, required := range []string{"/api/lookbook/render", "portraitImageBase64", "createObjectURL", "download", "1200", "1800"} {
		if !strings.Contains(s, required) {
			t.Errorf("lookbook must use shared Go compositor: %s", required)
		}
	}
	for _, bad := range []string{"fetch('https://", `fetch("https://`, "fetch('/api/tryon/render'"} {
		if strings.Contains(s, bad) {
			t.Errorf("lookbook must not call external renderer: %s", bad)
		}
	}
	if got := testApp(t).preflightTryon(tryonPreflightInput{}).LookbookLayout; got != "lookbook_card_v1" {
		t.Fatalf("unexpected preflight layout: %q", got)
	}
}

func TestLookbookMCPWorkflowContainsTwoStageRendering(t *testing.T) {
	a := testApp(t)
	flow := a.workflowForMCP("wardrobe-outfits")
	for _, keyword := range []string{"lookbook_card_v1", "右侧", "compose_wardrobe_lookbook", "人物图生成阶段"} {
		if !strings.Contains(flow, keyword) {
			t.Errorf("MCP workflow missing two-stage layout requirement: %q", keyword)
		}
	}
}
