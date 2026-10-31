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

func TestExtensionUIAccessBoundaries(t *testing.T) {
	a := testApp(t)
	secret, err := a.s.orderImportKey()
	if err != nil {
		t.Fatal(err)
	}
	origin := "chrome-extension://" + strings.Repeat("a", 32)
	run := func(method, path, from, key string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, "http://127.0.0.1:17936"+path, nil)
		if from != "" {
			r.Header.Set("Origin", from)
			r.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		if key != "" {
			r.Header.Set("X-Wardrobe-Import-Key", key)
		}
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	// Default Web UI still works without extension credentials.
	if got := run("GET", "/api/health", "", ""); got.Code != 200 {
		t.Fatalf("web UI broken: %d", got.Code)
	}
	if got := run("GET", "/api/products", origin, ""); got.Code != 403 {
		t.Fatalf("unpaired extension allowed: %d", got.Code)
	}
	if got := run("GET", "/api/products", origin, "wrong"); got.Code != 403 {
		t.Fatalf("wrong key accepted: %d", got.Code)
	}
	if got := run("GET", "/api/products", "https://evil.example", secret); got.Code != 403 {
		t.Fatalf("foreign origin accepted: %d", got.Code)
	}
	if got := run("GET", "/api/products", origin, secret); got.Code != 200 {
		t.Fatalf("paired extension cannot load products: %d", got.Code)
	}
	for _, endpoint := range []string{"/api/orders/pairing", "/mcp", "/"} {
		if got := run("GET", endpoint, origin, secret); got.Code != 403 {
			t.Fatalf("unsafe extension route %s => %d", endpoint, got.Code)
		}
	}
	r := httptest.NewRequest("OPTIONS", "http://127.0.0.1:17936/api/orders", nil)
	r.Header.Set("Origin", origin)
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "X-Wardrobe-Import-Key")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != origin {
		t.Fatalf("extension preflight invalid: %d", w.Code)
	}
	if got := run("GET", "/api/export", origin, secret); got.Code != 200 {
		t.Fatalf("paired local backup UI blocked: %d", got.Code)
	}
}

func TestUnifiedLocalePersistsAcrossHosts(t *testing.T) {
	a := testApp(t)
	key, err := a.s.orderImportKey()
	if err != nil {
		t.Fatal(err)
	}
	extOrigin := "chrome-extension://" + strings.Repeat("b", 32)
	for _, s := range []string{"auto", "en", "zh-CN"} {
		r := httptest.NewRequest("PUT", "http://127.0.0.1:17936/api/ui-locale", strings.NewReader(`{"uiLocale":"`+s+`"}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("locale %s not saved: %d", s, w.Code)
		}
		r = httptest.NewRequest("GET", "http://127.0.0.1:17936/api/ui-locale", nil)
		r.Header.Set("Origin", extOrigin)
		r.Header.Set("X-Wardrobe-Import-Key", key)
		w = httptest.NewRecorder()
		a.ServeHTTP(w, r)
		var data struct {
			UILocale string `json:"uiLocale"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &data) != nil || data.UILocale != s {
			t.Fatalf("extension locale differs: %s %d", data.UILocale, w.Code)
		}
	}
	r := httptest.NewRequest("PUT", "http://127.0.0.1:17936/api/ui-locale", strings.NewReader(`{"uiLocale":"unsupported"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("invalid locale accepted")
	}
	a.s.db.Close()
	other, err := newStore(a.s.dir)
	if err != nil {
		t.Fatal(err)
	}
	a.s = other
	value, err := a.s.appLocale()
	if err != nil || value != "zh-CN" {
		t.Fatal("locale did not persist", err)
	}
}

func TestMediaImagesRequirePairingInExtensionContext(t *testing.T) {
	a := testApp(t)
	dir := filepath.Join(a.s.dir, "images")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "W0001.jpg"), []byte("fake image fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	origin := "chrome-extension://" + strings.Repeat("a", 32)
	req := func(secret string) int {
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:17936/media/images/W0001.jpg", nil)
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Wardrobe-Import-Key", secret)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	if req("") != 403 {
		t.Fatal("media readable without pairing")
	}
	secret, _ := a.s.orderImportKey()
	if req(secret) != 200 {
		t.Fatal("paired extension cannot load its archived images")
	}
}
