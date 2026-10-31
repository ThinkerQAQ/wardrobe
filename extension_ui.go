package main

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Browser extension and local Web UI have the same feature APIs, backed by
// exactly one SQLite database. Only paired local extensions may call these
// endpoints. Never expose MCP or the pairing key to the extension origin.
func (a *application) prepareExtensionUI(w http.ResponseWriter, r *http.Request) (bool, bool) {
	origin := r.Header.Get("Origin")
	if !extensionOrigin.MatchString(origin) {
		return false, false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Wardrobe-Import-Key")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, DELETE, OPTIONS")
	w.Header().Add("Vary", "Origin")
	w.Header().Set("Cache-Control", "no-store")
	path := r.URL.Path
	// This is deliberately narrower than the entire local server.
	if !(strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/media/")) || path == "/api/orders/pairing" || path == "/mcp" {
		http.Error(w, "not available to extension", 403)
		return true, false
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return true, false
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "POST" && r.Method != "PUT" && r.Method != "DELETE" {
		http.Error(w, "method not allowed", 405)
		return true, false
	}
	key, err := a.s.orderImportKey()
	if err != nil {
		http.Error(w, "local pairing not ready", 500)
		return true, false
	}
	given := r.Header.Get("X-Wardrobe-Import-Key")
	if len(given) != len(key) || subtle.ConstantTimeCompare([]byte(given), []byte(key)) != 1 {
		http.Error(w, "pairing required", 403)
		return true, false
	}
	return true, true
}

func (s *store) appLocale() (string, error) {
	var locale string
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key='ui_locale'`).Scan(&locale)
	if errors.Is(err, sql.ErrNoRows) {
		return "auto", nil
	}
	return locale, err
}
func (s *store) saveAppLocale(locale string) error {
	if locale != "auto" && locale != "zh-CN" && locale != "en" {
		return errors.New("ui locale must be auto, zh-CN, or en")
	}
	_, err := s.db.Exec(`INSERT INTO app_settings(key,value) VALUES('ui_locale',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, locale)
	return err
}
func (a *application) serveLocale(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		locale, err := a.s.appLocale()
		if err != nil {
			fail(w, 500, err)
			return
		}
		jsonOut(w, 200, map[string]string{"uiLocale": locale})
	case "PUT":
		var in struct {
			UiLocale string `json:"uiLocale"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			fail(w, 400, err)
			return
		}
		if err := a.s.saveAppLocale(in.UiLocale); err != nil {
			fail(w, 400, err)
			return
		}
		jsonOut(w, 200, map[string]string{"uiLocale": in.UiLocale})
	default:
		http.Error(w, "method not allowed", 405)
	}
}
