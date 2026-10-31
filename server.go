package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

type application struct {
	s    *store
	ui   http.Handler
	host string
	mcp  http.Handler
}

func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, e error) {
	jsonOut(w, status, map[string]string{"error": e.Error()})
}
func (a *application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// Only the full local application entry can be embedded in a Chrome/Edge
	// extension side panel. Other routes keep X-Frame-Options DENY and
	// frame-ancestors 'none', preventing ordinary website clickjacking.
	embedded := r.URL.Path == "/" && r.URL.Query().Get("sidepanel") == "1" && r.Header.Get("Sec-Fetch-Dest") == "iframe"
	if embedded {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors chrome-extension:")
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' blob:; style-src 'self'; script-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	}
	if r.Host != a.host {
		http.Error(w, "invalid host", 403)
		return
	}
	// A paired extension shares the local feature APIs, but never accesses
	// the pairing secret or the MCP endpoint via this route.
	if handled, allowed := a.prepareExtensionUI(w, r); handled && !allowed {
		return
	}
	if o := r.Header.Get("Origin"); o != "" && o != "http://"+a.host && !extensionOrigin.MatchString(o) {
		http.Error(w, "cross-origin request denied", 403)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" && r.Header.Get("Sec-Fetch-Mode") != "navigate" && !extensionOrigin.MatchString(r.Header.Get("Origin")) {
		http.Error(w, "cross-site request denied", 403)
		return
	}
	if p := r.URL.Path; p == "/api/orders/import-file" && r.Method == "POST" {
		r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
		a.serveOrderFile(w, r)
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "application/json required", 415)
			return
		}
		limit := int64(2 << 20)
		if r.URL.Path == "/mcp" || r.URL.Path == "/api/lookbook/render" {
			limit = 12 << 20
		} // Authenticated image import needs room for Base64.
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	p := r.URL.Path
	if p == "/api/orders/ingest" {
		a.serveOrdersExtension(w, r)
		return
	}
	if p == "/mcp" && a.mcp != nil {
		a.mcp.ServeHTTP(w, r)
		return
	}
	if p == "/api/ui-locale" {
		a.serveLocale(w, r)
		return
	}
	if p == "/api/capabilities" && r.Method == "GET" {
		jsonOut(w, 200, map[string]bool{"retailerSync": retailerSyncEnabled, "remoteOrderImages": len(allowedOrderImageHosts) > 0})
		return
	}
	if p == "/api/health" {
		jsonOut(w, 200, map[string]string{"status": "ok", "app": "wardrobe"})
		return
	}
	if p == "/api/export" && r.Method == "GET" {
		out := map[string]any{"schemaVersion": 1}
		for _, kind := range []string{"products", "looks", "notes", "hairstyles", "body", "skill_overrides", "tryon_evidence"} {
			rows, e := a.s.list(kind)
			if e != nil {
				fail(w, 500, e)
				return
			}
			out[kind] = rows
		}
		orders, e := a.s.orders()
		if e != nil {
			fail(w, 500, e)
			return
		}
		out["orders"] = orders
		w.Header().Set("Content-Disposition", "attachment; filename=wardrobe-backup.json")
		jsonOut(w, 200, out)
		return
	}
	if p == "/api/orders/pairing" && r.Method == "GET" {
		key, e := a.s.orderImportKey()
		if e != nil {
			fail(w, 500, e)
			return
		}
		jsonOut(w, 200, map[string]string{"key": key})
		return
	}
	if p == "/api/orders" || strings.HasPrefix(p, "/api/orders/") {
		a.serveOrders(w, r)
		return
	}
	if p == "/api/lookbook/render" {
		a.serveLookbook(w, r)
		return
	}
	if strings.HasPrefix(p, "/api/tryon/") {
		a.serveTryon(w, r)
		return
	}
	if p == "/api/skills" || strings.HasPrefix(p, "/api/skills/") {
		a.serveSkills(w, r)
		return
	}
	if strings.HasPrefix(p, "/api/") {
		a.api(w, r)
		return
	}
	if strings.HasPrefix(p, "/media/") && (r.Method == "GET" || r.Method == "HEAD") {
		parts := strings.Split(strings.TrimPrefix(p, "/media/"), "/")
		if len(parts) != 2 || (parts[0] != "images" && parts[0] != "photos" && parts[0] != "hairstyles") || !validID.MatchString(strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))) {
			http.NotFound(w, r)
			return
		}
		ext := strings.ToLower(filepath.Ext(parts[1]))
		if ext != ".jpg" && ext != ".png" && ext != ".webp" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "private, max-age=86400")
		http.ServeFile(w, r, filepath.Join(a.s.dir, parts[0], parts[1]))
		return
	}
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	a.ui.ServeHTTP(w, r)
}
func (a *application) api(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 {
		http.NotFound(w, r)
		return
	}
	kind := parts[1]
	if kind != "products" && kind != "looks" && kind != "notes" && kind != "hairstyles" && kind != "body" {
		http.NotFound(w, r)
		return
	}
	id := ""
	if len(parts) == 3 {
		id = parts[2]
	}
	if r.Method == "GET" {
		if id != "" {
			v, e := a.s.get(kind, id)
			if e != nil {
				fail(w, 404, e)
				return
			}
			jsonOut(w, 200, v)
			return
		}
		rows, e := a.s.list(kind)
		if e != nil {
			fail(w, 500, e)
			return
		}
		q := strings.ToLower(r.URL.Query().Get("q"))
		cat := r.URL.Query().Get("category")
		if q != "" || cat != "" {
			filtered := []record{}
			for _, row := range rows {
				b, _ := json.Marshal(row)
				if (q == "" || strings.Contains(strings.ToLower(string(b)), q)) && (cat == "" || row["category"] == cat) {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}
		jsonOut(w, 200, rows)
		return
	}
	if kind == "body" && r.Method == "PUT" && id != "" {
		var incoming record
		d := json.NewDecoder(r.Body)
		if e := d.Decode(&incoming); e != nil {
			fail(w, 400, e)
			return
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			http.Error(w, "one JSON object required", 400)
			return
		}
		version, ok := incoming["version"].(float64)
		if !ok || version < 1 || version != float64(int(version)) {
			http.Error(w, "positive version required", 400)
			return
		}
		current, e := a.s.get("body", id)
		if e != nil {
			fail(w, 404, e)
			return
		}
		values, ok := incoming["measurements"].(map[string]any)
		if !ok {
			http.Error(w, "measurements object required", 400)
			return
		}
		definitions, _ := current["measurements"].(map[string]any)
		history, _ := current["history"].([]any)
		for key, raw := range values {
			entry, defined := definitions[key].(map[string]any)
			if !defined {
				http.Error(w, "unknown measurement key", 400)
				return
			}
			number, yes := raw.(float64)
			if !yes || number <= 0 || number > 500 {
				http.Error(w, "measurement out of range", 400)
				return
			}
			if entry["value"] == number {
				continue
			}
			history = append(history, map[string]any{"key": key, "previous": entry["value"], "value": number, "at": time.Now().UTC().Format(time.RFC3339), "source": "用户在 Web UI 修改"})
			entry["value"] = number
			entry["updatedBy"] = "user"
		}
		current["measurements"] = definitions
		current["history"] = history
		saved, e := a.s.save("body", id, current, int(version))
		if e != nil {
			status := 500
			if errors.Is(e, errConflict) {
				status = 409
			}
			fail(w, status, e)
			return
		}
		jsonOut(w, 200, saved)
		return
	}
	if r.Method == "DELETE" {
		if kind != "looks" || id == "" {
			http.Error(w, "only DELETE /api/looks/{id} supported", 405)
			return
		}
		var incoming struct {
			Version int `json:"version"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&incoming); e != nil {
			fail(w, 400, e)
			return
		}
		var extra any
		if d.Decode(&extra) != io.EOF || incoming.Version < 1 {
			http.Error(w, "one JSON object with positive version required", 400)
			return
		}
		if e := a.s.deleteLook(id, incoming.Version); e != nil {
			status := 500
			if errors.Is(e, errConflict) {
				status = 409
			}
			fail(w, status, e)
			return
		}
		jsonOut(w, 200, map[string]any{"deleted": true, "id": id})
		return
	}
	if kind == "notes" || (r.Method != "PUT" && r.Method != "POST") {
		http.Error(w, "method not allowed", 405)
		return
	}
	var incoming record
	d := json.NewDecoder(r.Body)
	if e := d.Decode(&incoming); e != nil {
		fail(w, 400, e)
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "one JSON object required", 400)
		return
	}
	version := 0
	if v, ok := incoming["version"].(float64); ok {
		version = int(v)
	}
	var body record
	if r.Method == "POST" {
		if kind != "looks" || id != "" {
			http.Error(w, "only POST /api/looks supported", 400)
			return
		}
		b := make([]byte, 8)
		if _, e := rand.Read(b); e != nil {
			fail(w, 500, e)
			return
		}
		id = "LOOK-" + hex.EncodeToString(b)
		body = record{}
	} else {
		if id == "" || version < 1 {
			http.Error(w, "id and version required", 400)
			return
		}
		var e error
		body, e = a.s.get(kind, id)
		if e != nil {
			fail(w, 404, e)
			return
		}
	}
	allowed := []string{"rating", "ratingSource", "fit", "owned", "comment"}
	if kind == "looks" {
		allowed = []string{"name", "scene", "rating", "comment", "slots"}
	}
	for _, key := range allowed {
		if v, ok := incoming[key]; ok {
			body[key] = v
		}
	}
	if v, ok := body["rating"]; ok {
		n, isNumber := v.(float64)
		if !isNumber || n < 0 || n > 5 || n != float64(int(n)) {
			http.Error(w, "rating must be 0..5", 400)
			return
		}
	}
	if kind == "products" {
		body["ratingSource"] = "user"
	}
	if slots, ok := body["slots"].(map[string]any); ok {
		validSlots := map[string]bool{"outer": true, "inner": true, "mid": true, "pants": true, "socks": true, "shoes": true, "glasses": true, "hat": true, "gloves": true, "necklace": true, "watch": true, "belt": true, "scarf": true, "bag": true, "tie": true, "accessory1": true, "accessory2": true, "other": true, "hair": true}
		for key, v := range slots {
			pid, isString := v.(string)
			if !validSlots[key] || !isString {
				http.Error(w, "invalid slots", 400)
				return
			}
			if pid != "" {
				lookupKind := "products"
				if key == "hair" {
					lookupKind = "hairstyles"
				}
				if _, e := a.s.get(lookupKind, pid); errors.Is(e, sql.ErrNoRows) {
					http.Error(w, "unknown product: "+pid, 400)
					return
				} else if e != nil {
					fail(w, 500, e)
					return
				}
			}
		}
	} else if kind == "looks" {
		http.Error(w, "slots object required", 400)
		return
	}
	for _, k := range []string{"name", "scene", "comment", "fit", "owned", "ratingSource"} {
		if v, ok := body[k]; ok {
			str, yes := v.(string)
			if !yes || len(str) > 20000 {
				http.Error(w, "invalid text field", 400)
				return
			}
		}
	}
	result, e := a.s.save(kind, id, body, version)
	if e != nil {
		status := 500
		if errors.Is(e, errConflict) {
			status = 409
		}
		fail(w, status, e)
		return
	}
	jsonOut(w, 200, result)
}

var _ = url.URL{}
