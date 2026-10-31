package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testApp(t *testing.T) *application {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "seed.json"), []byte(`{"products":[{"id":"W0001","name":"shirt","rating":3,"fields":{"original":"keep"}}],"looks":[],"notes":[]}`), 0600)
	s, e := newStore(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.db.Close() })
	return &application{s: s, host: "127.0.0.1:17936", ui: http.NotFoundHandler()}
}
func request(a *application, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1:17936"+path, strings.NewReader(body))
	if method != "GET" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	return w
}
func TestPersistenceConflictAndValidation(t *testing.T) {
	a := testApp(t)
	w := request(a, "POST", "/api/looks", `{"name":"test","slots":{"inner":"W0001"},"rating":5}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var look record
	json.Unmarshal(w.Body.Bytes(), &look)
	a.s.db.Close()
	s, e := newStore(a.s.dir)
	if e != nil {
		t.Fatal(e)
	}
	a.s = s
	defer s.db.Close()
	saved, e := s.get("looks", look["id"].(string))
	if e != nil || saved["name"] != "test" {
		t.Fatal(saved, e)
	}
	w = request(a, "PUT", "/api/products/W0001", `{"version":1,"rating":4,"fields":{"original":"overwrite"}}`)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	p, _ := s.get("products", "W0001")
	if p["fields"].(map[string]any)["original"] != "keep" {
		t.Fatal("original fields overwritten")
	}
	w = request(a, "PUT", "/api/products/W0001", `{"version":1,"rating":2}`)
	if w.Code != 409 {
		t.Fatal("missing stale-write guard", w.Code)
	}
	w = request(a, "POST", "/api/looks", `{"slots":{"inner":"MISSING"}}`)
	if w.Code != 400 {
		t.Fatal("invalid association accepted")
	}
	w = request(a, "POST", "/api/looks", `{"slots":{},"rating":6}`)
	if w.Code != 400 {
		t.Fatal("invalid rating accepted")
	}
	w = request(a, "GET", "/api/export", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "schemaVersion") {
		t.Fatal("export failed")
	}
}
func TestSecurityBoundary(t *testing.T) {
	a := testApp(t)
	for _, kind := range []string{"host", "origin", "content"} {
		r := httptest.NewRequest("PUT", "http://127.0.0.1:17936/api/products/W0001", strings.NewReader(`{"version":1,"rating":5}`))
		r.Header.Set("Content-Type", "application/json")
		switch kind {
		case "host":
			r.Host = "evil.example"
		case "origin":
			r.Header.Set("Origin", "https://evil.example")
		case "content":
			r.Header.Set("Content-Type", "text/plain")
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal(kind + " accepted")
		}
	}
}

func TestLookLayersAndDelete(t *testing.T) {
	a := testApp(t)
	// Keep existing inner slot compatible; newly supported middle layer and socks are independent.
	w := request(a, "POST", "/api/looks", `{"name":"layered","slots":{"inner":"W0001","mid":"W0001","socks":"W0001"},"rating":0}`)
	if w.Code != 200 {
		t.Fatal("create layers", w.Code, w.Body.String())
	}
	var saved record
	if e := json.Unmarshal(w.Body.Bytes(), &saved); e != nil {
		t.Fatal(e)
	}
	id := saved["id"].(string)
	if saved["slots"].(map[string]any)["inner"] != "W0001" {
		t.Fatal("legacy inner lost")
	}
	w = request(a, "PUT", "/api/looks/"+id, `{"version":1,"slots":{"inner":"W0001","mid":"W0001","socks":"W0001","glasses":"W0001","hat":"W0001","gloves":"W0001","necklace":"W0001","watch":"W0001","belt":"W0001","scarf":"W0001","bag":"W0001","tie":"W0001"},"name":"edited"}`)
	if w.Code != 200 {
		t.Fatal("update", w.Code, w.Body.String())
	}
	updated, e := a.s.get("looks", id)
	if e != nil || updated["slots"].(map[string]any)["watch"] != "W0001" {
		t.Fatal("accessories were not persisted", e)
	}
	w = request(a, "DELETE", "/api/looks/"+id, `{"version":1}`)
	if w.Code != 409 {
		t.Fatal("stale delete should conflict", w.Code, w.Body.String())
	}
	if _, e := a.s.get("looks", id); e != nil {
		t.Fatal("stale deletion removed record", e)
	}
	w = request(a, "DELETE", "/api/products/W0001", `{"version":1}`)
	if w.Code != 405 {
		t.Fatal("product delete allowed", w.Code)
	}
	w = request(a, "DELETE", "/api/looks/"+id, `{"version":0}`)
	if w.Code != 400 {
		t.Fatal("versionless delete allowed", w.Code)
	}
	w = request(a, "DELETE", "/api/looks/"+id, `{"version":2}`)
	if w.Code != 200 {
		t.Fatal("valid delete", w.Code, w.Body.String())
	}
	w = request(a, "GET", "/api/looks/"+id, "")
	if w.Code != 404 {
		t.Fatal("look still exists", w.Code)
	}
	w = request(a, "GET", "/api/products/W0001", "")
	if w.Code != 200 {
		t.Fatal("deleting look affected a product", w.Code)
	}
	w = request(a, "DELETE", "/api/looks/"+id, `{"version":2}`)
	if w.Code != 409 {
		t.Fatal("duplicate deletion should conflict", w.Code)
	}
	w = request(a, "POST", "/api/looks", `{"slots":{"unknown":"W0001"}}`)
	if w.Code != 400 {
		t.Fatal("unexpected slot accepted", w.Code)
	}
}
