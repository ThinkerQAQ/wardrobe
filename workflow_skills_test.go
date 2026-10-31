package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkflowEditorOverridesAndRestore(t *testing.T) {
	a := testApp(t)
	name := "wardrobe-aesthetics"
	defaultView, e := a.s.getWorkflow(name)
	if e != nil || defaultView.Modified || defaultView.Version != 0 || !strings.Contains(defaultView.Content, "name: wardrobe-aesthetics") {
		t.Fatalf("default workflow: %+v, %v", defaultView, e)
	}
	list := request(a, "GET", "/api/skills", "")
	if list.Code != 200 {
		t.Fatal("list skills", list.Code, list.Body.String())
	}
	var all []workflowView
	if e = json.Unmarshal(list.Body.Bytes(), &all); e != nil || len(all) != 4 {
		t.Fatal(e, len(all))
	}
	if all[0].Content != "" {
		t.Fatal("list should not expose entire source")
	}
	updatedContent := defaultView.Content + "\n## Edited locally\nUser-selected note, do not change clothes.\n"
	body, _ := json.Marshal(map[string]any{"version": 0, "content": updatedContent})
	result := request(a, "PUT", "/api/skills/"+name, string(body))
	if result.Code != 200 {
		t.Fatal(result.Code, result.Body.String())
	}
	current, e := a.s.getWorkflow(name)
	if e != nil || !current.Modified || current.Version != 1 || current.Content != updatedContent {
		t.Fatal(current, e)
	}
	// The same function provides the active content to the MCP workflow tool.
	out := a.workflowForMCP(name)
	if !strings.Contains(out, "Edited locally") {
		t.Fatal("MCP reader did not see override")
	}
	if !strings.Contains(out, "SQLite 实时审美记录") {
		t.Fatal("lost live notes section")
	}
	stale := request(a, "PUT", "/api/skills/"+name, string(body))
	if stale.Code != 409 {
		t.Fatal("stale edit should fail", stale.Code)
	}
	bad := strings.Replace(updatedContent, "name: wardrobe-aesthetics", "name: wardrobe-import", 1)
	invalid, _ := json.Marshal(map[string]any{"version": 1, "content": bad})
	if r := request(a, "PUT", "/api/skills/"+name, string(invalid)); r.Code != 400 {
		t.Fatal("identity change allowed", r.Code)
	}
	// No extra writes; the reset only deletes the local override, not the default.
	reset := request(a, "DELETE", "/api/skills/"+name, `{"version":1}`)
	if reset.Code != 200 {
		t.Fatal("reset", reset.Code, reset.Body.String())
	}
	after, e := a.s.getWorkflow(name)
	if e != nil || after.Modified || after.Content != defaultView.Content {
		t.Fatal("default not restored", e)
	}
	if r := request(a, "DELETE", "/api/skills/"+name, `{"version":1}`); r.Code != 409 {
		t.Fatal("repeated reset", r.Code)
	}
	if _, e = a.s.get("notes", "NOTE-TEST"); e == nil {
		t.Fatal("test unexpectedly populated notes")
	}
	result = request(a, "GET", "/api/export", "")
	if !strings.Contains(result.Body.String(), "skill_overrides") {
		t.Fatal("backup excludes skill edits")
	}
}

func TestWorkflowIdentityAndInputValidation(t *testing.T) {
	a := testApp(t)
	for _, path := range []string{"/api/skills/../notes", "/api/skills/none", "/api/skills/wardrobe-import/extra"} {
		r := request(a, "GET", path, "")
		if r.Code != 404 {
			t.Fatal("unexpected skill path", path, r.Code)
		}
	}
	content, e := a.s.getWorkflow("wardrobe-import")
	if e != nil {
		t.Fatal(e)
	}
	testCases := []struct {
		version any
		content string
	}{
		{0, "plain text"},
		{-1, content.Content},
		{0, strings.Repeat("x", 65537)},
		{0, strings.Replace(content.Content, "description:", "other:", 1)},
	}
	for _, tt := range testCases {
		raw, _ := json.Marshal(map[string]any{"version": tt.version, "content": tt.content})
		r := request(a, "PUT", "/api/skills/wardrobe-import", string(raw))
		if r.Code != 400 {
			t.Fatalf("invalid input accepted: %d %s", r.Code, r.Body.String())
		}
	}
}

func TestBundledSkillFrontmatterAndImportReference(t *testing.T) {
	a := testApp(t)
	for _, name := range workflowNames {
		view, err := a.s.getWorkflow(name)
		if err != nil {
			t.Fatal(name, err)
		}
		if err := validateSkillContent(name, view.Content); err != nil {
			t.Fatalf("invalid bundled Skill %s: %v", name, err)
		}
	}
	content := a.workflowForMCP("wardrobe-import")
	if !strings.Contains(content, "订单核验补充说明（随程序内置）") ||
		!strings.Contains(content, "不能把退款当退货") {
		t.Fatal("import reference guide was not included in MCP workflow")
	}
	if strings.Contains(content, "用户原始资料，仅供事实参考") {
		t.Fatal("import workflow must not append personal aesthetic notes")
	}
}

func TestTryOnSkillSpecBundledIntoMCP(t *testing.T) {
	a := testApp(t)
	view, err := a.s.getWorkflow("wardrobe-outfits")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSkillContent("wardrobe-outfits", view.Content); err != nil {
		t.Fatal(err)
	}
	mcpWorkflow := a.workflowForMCP("wardrobe-outfits")
	required := []string{
		"真人 AI 试穿验收补充规范（随程序内置）",
		"identity_photo",
		"image_input_verified",
		"accessory_allowlist",
		"手提包",
		"正脸",
		"双眼",
		"三分之二侧脸",
		"多宫格",
		"Q1 构图",
		"Q2 面容",
		"Q4 配饰",
		"NO bag",
		"不提供能保证",
	}
	for _, word := range required {
		if !strings.Contains(mcpWorkflow, word) {
			t.Errorf("MCP try-on instructions missing mandatory rule %q", word)
		}
	}
	if strings.Contains(view.Content, "用户原始资料，仅供事实参考") {
		t.Fatal("try-on skill must not embed personal notes")
	}
	// Custom user Skills must still get the release safety checklist when sent over MCP.
	edited := view.Content + "\n## 用户自定义\n保留现有衣服颜色。\n"
	_, err = a.s.updateWorkflow("wardrobe-outfits", edited, 0)
	if err != nil {
		t.Fatal(err)
	}
	overridden := a.workflowForMCP("wardrobe-outfits")
	if !strings.Contains(overridden, "用户自定义") || !strings.Contains(overridden, "Q4 配饰") {
		t.Fatal("the overriden workflow lost user changes or the mandatory try-on checklist")
	}
}
