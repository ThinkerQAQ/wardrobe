package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var extensionOrigin = regexp.MustCompile(`^chrome-extension://[a-p]{32}$`)

// The shared Wardrobe extension uses a separate local pairing key (legacy
// filename order-import.key). Never expose Cookie values or the MCP tunnel key.
func (s *store) orderImportKey() (string, error) {
	path := filepath.Join(s.dir, "order-import.key")
	if b, e := os.ReadFile(path); e == nil {
		v := strings.TrimSpace(string(b))
		if len(v) == 64 {
			return v, nil
		}
		return "", errors.New("invalid local import key")
	} else if !os.IsNotExist(e) {
		return "", e
	}
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	v := hex.EncodeToString(b)
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(e) {
		return s.orderImportKey()
	}
	if e != nil {
		return "", e
	}
	if _, e = f.WriteString(v + "\n"); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	return v, nil
}
func (a *application) serveOrdersExtension(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if !extensionOrigin.MatchString(origin) {
		http.Error(w, "extension origin required", 403)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	if r.Method == "OPTIONS" {
		if r.Header.Get("Access-Control-Request-Method") != "POST" {
			http.Error(w, "unsupported preflight", 405)
			return
		}
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Wardrobe-Import-Key")
		w.WriteHeader(204)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "JSON required", 415)
		return
	}
	key, e := a.s.orderImportKey()
	if e != nil {
		fail(w, 500, e)
		return
	}
	supplied := r.Header.Get("X-Wardrobe-Import-Key")
	if len(supplied) != len(key) || subtle.ConstantTimeCompare([]byte(supplied), []byte(key)) != 1 {
		http.Error(w, "pairing key rejected", 403)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	var input OrderImport
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(&input); e != nil {
		fail(w, 400, errors.New("invalid normalized order payload"))
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "one JSON document required", 400)
		return
	}
	result, e := a.s.importOrders(input)
	if e != nil {
		fail(w, 400, e)
		return
	}
	jsonOut(w, 200, result)
}
