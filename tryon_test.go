package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tryonPhotoFixture(t *testing.T, a *application, lookID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(a.s.dir, "photos"), 0700); err != nil {
		t.Fatal(err)
	}
	im := image.NewRGBA(image.Rect(0, 0, 320, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 320; x++ {
			im.Set(x, y, color.RGBA{uint8((x * y) % 251), uint8((x + y) % 253), uint8(y % 254), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 78}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.s.dir, "photos", lookID+".jpg"), buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.s.save("looks", lookID, record{"name": "reference photo", "photo": "/media/photos/" + lookID + ".jpg", "slots": map[string]string{}}, 0); err != nil {
		t.Fatal(err)
	}
}

func TestTryonPreflightRejectsMissingProvenanceAndExtraObjects(t *testing.T) {
	a := testApp(t)
	if _, err := a.s.save("looks", "LOOK-TEST", record{"name": "business", "slots": map[string]string{"outer": "W0001", "pants": "W0001", "shoes": "W0001"}}, 0); err != nil {
		t.Fatal(err)
	}
	input := `{"lookId":"LOOK-TEST","faceLookId":"LOOK-REF","bodyLookId":"LOOK-REF"}`
	w := request(a, "POST", "/api/tryon/preflight", input)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var out tryonPreflightOutput
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Ready || out.Stage != "blocked" || len(out.Blocks) < 2 {
		t.Fatal("unverified inputs must be blocked", out)
	}
	if len(out.Accessories) != 0 {
		t.Fatal("unselected bag/watch must not be implicitly allowed", out.Accessories)
	}
	if !strings.Contains(out.Pose, "feet parallel") || !strings.Contains(out.Pose, "head upright") {
		t.Fatal("normal standing pose missing", out.Pose)
	}
	render := request(a, "POST", "/api/tryon/render", `{}`)
	if render.Code != 501 {
		t.Fatal("render must fail closed without validated provider", render.Code)
	}
	if x, err := a.s.list("tryon_evidence"); err != nil || len(x) != 0 {
		t.Fatal("preflight/render must not create any provenance", err)
	}
}

func TestTryonRequiresExplicitLocalPhotoVerificationAndInvalidatesOnChange(t *testing.T) {
	a := testApp(t)
	tryonPhotoFixture(t, a, "LOOK-REF")
	if err := os.MkdirAll(filepath.Join(a.s.dir, "images"), 0700); err != nil {
		t.Fatal(err)
	}
	actualPhoto, err := os.ReadFile(filepath.Join(a.s.dir, "photos", "LOOK-REF.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.s.dir, "images", "W0001.jpg"), actualPhoto, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := a.s.get("products", "W0001")
	if err != nil {
		t.Fatal(err)
	}
	p["image"] = "/media/images/W0001.jpg"
	if _, err := a.s.save("products", "W0001", p, p["version"].(int)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.s.save("looks", "LOOK-TEST", record{"name": "business", "slots": map[string]string{"outer": "W0001", "pants": "W0001", "shoes": "W0001"}}, 0); err != nil {
		t.Fatal(err)
	}
	payload := `{"lookId":"LOOK-TEST","faceLookId":"LOOK-REF","bodyLookId":"LOOK-REF"}`
	if request(a, "POST", "/api/tryon/confirm-photo", `{"lookId":"LOOK-REF","role":"face","confirmRealPhoto":true}`).Code != 400 {
		t.Fatal("cannot confirm face without frontal user assertion")
	}
	if request(a, "POST", "/api/tryon/confirm-photo", `{"lookId":"LOOK-REF","role":"face","confirmRealPhoto":true,"confirmFrontal":true}`).Code != 200 {
		t.Fatal("frontal photo confirmation not saved")
	}
	first := request(a, "POST", "/api/tryon/preflight", payload)
	var one tryonPreflightOutput
	json.Unmarshal(first.Body.Bytes(), &one)
	if one.Ready {
		t.Fatal("body reference was never explicitly confirmed")
	}
	if request(a, "POST", "/api/tryon/confirm-photo", `{"lookId":"LOOK-REF","role":"body","confirmRealPhoto":true,"confirmFullBody":true}`).Code != 200 {
		t.Fatal("full body confirmation failed")
	}
	second := request(a, "POST", "/api/tryon/preflight", payload)
	var two tryonPreflightOutput
	if err := json.Unmarshal(second.Body.Bytes(), &two); err != nil {
		t.Fatal(err)
	}
	if !two.Ready || two.Stage != "ready_for_renderer_only" {
		t.Fatal("valid confirmed inputs must reach renderer boundary", two.Blocks)
	}
	if len(two.Accessories) != 0 || two.IdentityReferenceDigest == "" {
		t.Fatal("incorrect allowlist or missing digest", two)
	}
	if request(a, "POST", "/api/tryon/render", `{}`).Code != 501 {
		t.Fatal("render cannot be bypassed even after preflight")
	}
	// Replacing an archived photo cannot keep prior face/body approval.
	file := filepath.Join(a.s.dir, "photos", "LOOK-REF.jpg")
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("changed-image-content")); err != nil {
		t.Fatal(err)
	}
	f.Close()
	third := request(a, "POST", "/api/tryon/preflight", payload)
	var three tryonPreflightOutput
	json.Unmarshal(third.Body.Bytes(), &three)
	if three.Ready {
		t.Fatal("changed photograph must invalidate provenance")
	}
}

func TestTryonRequiresLocalHostAndRejectsFakePhoto(t *testing.T) {
	a := testApp(t)
	tryonPhotoFixture(t, a, "LOOK-REF")
	if w := request(a, "POST", "/api/tryon/confirm-photo", `{"lookId":"LOOK-REF","role":"face","confirmRealPhoto":false,"confirmFrontal":true}`); w.Code != 400 {
		t.Fatal("false provenance accepted", w.Code)
	}
	if w := request(a, "POST", "/api/tryon/confirm-photo", `{"lookId":"LOOK-REF","role":"face","confirmRealPhoto":true,"confirmFrontal":true,"extra":true}`); w.Code != 400 {
		t.Fatal("unknown fields accepted", w.Code)
	}
	if w := request(a, "GET", "/api/tryon/confirm-photo", ""); w.Code != 405 {
		t.Fatal("GET mutation accepted", w.Code)
	}
}
