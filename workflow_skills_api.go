package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// serveSkills is the explicit local Web UI workflow editor API. Editing a Skill
// never writes to Git files; data lives in the same local SQLite backup domain.
func (a *application) serveSkills(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimPrefix(r.URL.Path, "/api/skills")
	if raw == "" || raw == "/" {
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		rows := make([]workflowView, 0, len(workflowNames))
		for _, name := range workflowNames {
			item, e := a.s.getWorkflow(name)
			if e != nil {
				fail(w, 500, e)
				return
			}
			// The list only needs summaries, not four copies of complete instructions.
			item.Content = ""
			rows = append(rows, item)
		}
		jsonOut(w, 200, rows)
		return
	}
	name := strings.TrimPrefix(raw, "/")
	if strings.Contains(name, "/") || !isWorkflowName(name) {
		http.NotFound(w, r)
		return
	}
	if r.Method == "GET" {
		item, e := a.s.getWorkflow(name)
		if e != nil {
			fail(w, 500, e)
			return
		}
		jsonOut(w, 200, item)
		return
	}
	if r.Method != "PUT" && r.Method != "DELETE" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in struct {
		Version *int    `json:"version"`
		Content *string `json:"content,omitempty"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if e := dec.Decode(&in); e != nil {
		fail(w, 400, e)
		return
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || in.Version == nil {
		http.Error(w, "version required; one JSON object only", 400)
		return
	}
	var item workflowView
	var e error
	if r.Method == "PUT" {
		if in.Content == nil {
			http.Error(w, "content required", 400)
			return
		}
		item, e = a.s.updateWorkflow(name, *in.Content, *in.Version)
	} else {
		if in.Content != nil {
			http.Error(w, "reset must not include content", 400)
			return
		}
		item, e = a.s.resetWorkflow(name, *in.Version)
	}
	if e != nil {
		code := 400
		if errors.Is(e, errConflict) {
			code = 409
		} else if strings.Contains(e.Error(), "database") {
			code = 500
		}
		fail(w, code, e)
		return
	}
	jsonOut(w, 200, item)
}
