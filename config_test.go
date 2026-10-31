package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	c, e := loadConfig(path, false)
	if e != nil || c.Storage.DataDir != dir || !c.Server.OpenBrowser {
		t.Fatalf("defaults: %+v %v", c, e)
	}
	if _, e = loadConfig(path, true); e == nil {
		t.Fatal("explicit missing config accepted")
	}
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"[server]\naddress='127.0.0.1:17937'\nopen_browser=false\n[storage]\ndata_dir='private'", true},
		{"[server]\naddres='127.0.0.1:17937'", false},
		{"[storage]\ndata_dir=''", false},
		{"[server", false},
	} {
		os.WriteFile(path, []byte(tc.body), 0600)
		c, e = loadConfig(path, true)
		if (e == nil) != tc.valid {
			t.Fatalf("config %q: %v", tc.body, e)
		}
		if tc.valid && (c.Server.OpenBrowser || c.Storage.DataDir != filepath.Join(dir, "private")) {
			t.Fatalf("decode: %+v", c)
		}
	}
	for _, a := range []string{"0.0.0.0:17936", "localhost:0", "localhost:65536", "localhost:http"} {
		if validateAddress(a) == nil {
			t.Fatalf("accepted %s", a)
		}
	}
	if validateAddress("127.0.0.1:17936") != nil {
		t.Fatal("valid address rejected")
	}
}

func TestEmptyInstallation(t *testing.T) {
	s, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.db.Close()
	for _, kind := range []string{"products", "looks", "notes"} {
		rows, err := s.list(kind)
		if err != nil || len(rows) != 0 {
			t.Fatalf("%s: %v %v", kind, rows, err)
		}
	}
}
