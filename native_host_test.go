package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestNativeHostManifestAllowlist(t *testing.T) {
	id1 := strings.Repeat("a", 32)
	id2 := strings.Repeat("b", 32)
	first, err := buildNativeManifest("/bin/wardrobe", id1, nil)
	if err != nil || len(first.AllowedOrigins) != 1 || first.AllowedOrigins[0] != "chrome-extension://"+id1+"/" {
		t.Fatalf("first: %+v %v", first, err)
	}
	prior, _ := json.Marshal(first)
	second, err := buildNativeManifest("/new/path", id2, prior)
	if err != nil || len(second.AllowedOrigins) != 2 || second.Path != "/new/path" {
		t.Fatalf("updated: %+v %v", second, err)
	}
	if _, err := buildNativeManifest("/bin/wardrobe", id1, []byte(`{"name":"unrelated","type":"stdio"}`)); err == nil {
		t.Fatal("overwrote foreign native host manifest")
	}
	for _, value := range []string{"invalid", strings.Repeat("q", 32), strings.Repeat("a", 31)} {
		if _, err := buildNativeManifest("/bin/wardrobe", value, nil); err == nil {
			t.Fatal("allowed invalid extension ID")
		}
	}
}
func TestNativeHostRepairPreservesOrigin(t *testing.T) {
	id := strings.Repeat("a", 32)
	old := nativeHostManifest{Name: nativeHostName, Type: "stdio", Path: `C:\old\wardrobe-native-v1.0.3.exe`, AllowedOrigins: []string{"chrome-extension://" + id + "/"}}
	bytes, _ := json.Marshal(old)
	got, err := repairNativeHostExtensionID(bytes)
	if err != nil || got != id {
		t.Fatalf("repair id=%q err=%v", got, err)
	}
	for _, bad := range [][]byte{[]byte(`{}`), []byte(`{"name":"foreign","type":"stdio"}`), []byte(`{"name":"com.thinkerqaq.wardrobe","type":"stdio","allowed_origins":["https://evil/"]}`)} {
		if _, err := repairNativeHostExtensionID(bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestNativeHostMessageFraming(t *testing.T) {
	var buf bytes.Buffer
	if err := writeNativeMessage(&buf, map[string]any{"command": "status"}); err != nil {
		t.Fatal(err)
	}
	data, err := readNativeMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var message map[string]any
	if json.Unmarshal(data, &message) != nil || message["command"] != "status" {
		t.Fatal("invalid message")
	}
	var oversize bytes.Buffer
	_ = binary.Write(&oversize, binary.LittleEndian, uint32(maxNativeBytes+1))
	if _, err = readNativeMessage(&oversize); err == nil {
		t.Fatal("unbounded native message")
	}
	if _, err = readNativeMessage(bytes.NewReader(nil)); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}
func TestNativeHostRejectsUnknownCommands(t *testing.T) {
	var in, out bytes.Buffer
	_ = writeNativeMessage(&in, map[string]string{"command": "exec", "path": "/bin/sh"})
	if err := runWardrobeNativeHost(&in, &out); err != nil {
		t.Fatal(err)
	}
	b, err := readNativeMessage(&out)
	if err != nil {
		t.Fatal(err)
	}
	var reply struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(b, &reply) != nil || reply.OK || reply.Error != "unsupported command" {
		t.Fatalf("unexpected reply: %s", b)
	}
}
func TestNativeHostInvocation(t *testing.T) {
	for _, args := range [][]string{{"--native-host"}, {"chrome-extension://" + strings.Repeat("a", 32) + "/"}, {"edge-extension://" + strings.Repeat("b", 32)}} {
		if !isNativeHostInvocation(args) {
			t.Fatal(args)
		}
	}
	for _, args := range [][]string{nil, {"--open=false"}, {"http://example.com"}, {"chrome-extension://evil/"}} {
		if isNativeHostInvocation(args) {
			t.Fatal(args)
		}
	}
}
