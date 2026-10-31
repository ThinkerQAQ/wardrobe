package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSidePanelEmbeddingIsRestricted(t *testing.T) {
	a := testApp(t)
	frame := func(path, dest string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:17936"+path, nil)
		r.Header.Set("Sec-Fetch-Dest", dest)
		r.Header.Set("Sec-Fetch-Mode", "navigate")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w
	}
	allowed := frame("/?sidepanel=1", "iframe")
	if allowed.Code != 404 {
		t.Fatalf("side panel request failed: %d", allowed.Code)
	}
	if allowed.Header().Get("X-Frame-Options") != "" {
		t.Fatal("side panel still denies embedding")
	}
	if !strings.Contains(allowed.Header().Get("Content-Security-Policy"), "frame-ancestors chrome-extension:") {
		t.Fatal("missing restricted extension parent")
	}
	for _, item := range []struct{ path, dest string }{{"/", "iframe"}, {"/?sidepanel=1", "document"}, {"/api/orders", "iframe"}, {"/lookbook.html?sidepanel=1", "iframe"}} {
		result := frame(item.path, item.dest)
		if result.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("unexpected framing permission %s", item.path)
		}
		if !strings.Contains(result.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatalf("missing default framing deny %s", item.path)
		}
	}
}
