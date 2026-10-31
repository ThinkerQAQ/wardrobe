package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductImportImageAndAudit(t *testing.T) {
	a := testApp(t)
	in := CreateProductInput{SourceKey: "source:order-20261030:line1", Platform: "淘宝", OrderID: "order-20261030", Verification: "verified", Name: "蓝色衬衫", Category: "衬衫", Color: "蓝色", Size: "M", Date: "2026-10-30", Fields: map[string]string{"退款状态": "无退款"}}
	created, e := a.s.importProduct(in)
	if e != nil {
		t.Fatal(e)
	}
	id := created["id"].(string)
	if created["owned"] != "待确认" || created["rating"].(float64) != 0 || created["ratingSource"] != "unrated" {
		t.Fatal("unverified preference created", created)
	}
	if _, e = a.s.importProduct(in); e == nil {
		t.Fatal("duplicate imported")
	}
	in.SourceKey = "source:order2:line1"
	in.OrderID = ""
	if _, e = a.s.importProduct(in); e == nil {
		t.Fatal("missing order accepted")
	}
	// Update only specified field, preserve the order evidence and rating.
	newName := "棉蓝色衬衫"
	updated, e := a.s.patchProduct(ProductPatchInput{ID: id, Version: 1, Name: &newName, Fields: map[string]string{"尺码核验": "待确认"}})
	if e != nil {
		t.Fatal(e)
	}
	fields := updated["fields"].(map[string]any)
	if fields["退款状态"] != "无退款" || fields["尺码核验"] != "待确认" || updated["ratingSource"] != "unrated" {
		t.Fatal("lost provenance", updated)
	}
	if _, e = a.s.patchProduct(ProductPatchInput{ID: id, Version: 1, Name: &newName}); e == nil {
		t.Fatal("stale edit accepted")
	}
	im := image.NewRGBA(image.Rect(0, 0, 640, 640))
	for y := 0; y < 640; y++ {
		for x := 0; x < 640; x++ {
			im.Set(x, y, color.RGBA{R: 50, G: 80, B: 150, A: 255})
		}
	}
	var buf bytes.Buffer
	if e = jpeg.Encode(&buf, im, &jpeg.Options{Quality: 85}); e != nil {
		t.Fatal(e)
	}
	saved, e := a.s.saveProductImage(SaveImageInput{ID: id, Version: 2, ImageBase64: base64.StdEncoding.EncodeToString(buf.Bytes()), Source: "合法来源实物图"})
	if e != nil {
		t.Fatal(e)
	}
	if saved.Width != 640 || saved.Version != 3 {
		t.Fatal(saved)
	}
	if _, e = os.Stat(filepath.Join(a.s.dir, "images", id+".jpg")); e != nil {
		t.Fatal(e)
	}
	audit, e := a.s.auditWardrobe()
	if e != nil {
		t.Fatal(e)
	}
	if audit.ProductCount != 2 || len(audit.MissingImages) != 1 {
		t.Fatal(audit)
	}
	if _, e = a.s.saveProductImage(SaveImageInput{ID: id, Version: 1, ImageBase64: base64.StdEncoding.EncodeToString(buf.Bytes()), Source: "stale"}); e == nil {
		t.Fatal("stale image update accepted")
	}
	if _, e = a.s.saveProductImage(SaveImageInput{ID: id, Version: 3, ImageBase64: base64.StdEncoding.EncodeToString([]byte("bad")), Source: "bad"}); e == nil {
		t.Fatal("invalid image accepted")
	}
}

func TestBodyHairstylePersistenceAndValidation(t *testing.T) {
	a := testApp(t)
	initial := record{"name": "身体数据", "measurements": map[string]any{"chest": map[string]any{"value": 91.0, "label": "胸围", "unit": "cm"}}, "garmentTargets": map[string]any{"top_chest": map[string]any{"min": 113.0, "max": 113.0, "unit": "cm"}}, "source": "衣服.md", "history": []any{}}
	if _, e := a.s.save("body", "BODY", initial, 0); e != nil {
		t.Fatal(e)
	}
	if _, e := a.s.save("hairstyles", "HAIR0001", record{"name": "短背头", "image": "/media/hairstyles/HAIR0001.jpg"}, 0); e != nil {
		t.Fatal(e)
	}
	w := request(a, "POST", "/api/looks", `{"name":"有发型","slots":{"hair":"HAIR0001","inner":"W0001"}}`)
	if w.Code != 200 {
		t.Fatal("hair slot rejected", w.Code, w.Body.String())
	}
	w = request(a, "POST", "/api/looks", `{"name":"未知发型","slots":{"hair":"HAIR0002"}}`)
	if w.Code != 400 {
		t.Fatal("unknown hairstyle accepted")
	}
	w = request(a, "PUT", "/api/body/BODY", `{"version":1,"measurements":{"chest":92}}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body, e := a.s.get("body", "BODY")
	if e != nil {
		t.Fatal(e)
	}
	if body["measurements"].(map[string]any)["chest"].(map[string]any)["value"].(float64) != 92 {
		t.Fatal("body not updated", body)
	}
	if len(body["history"].([]any)) != 1 {
		t.Fatal("measurements history lost", body)
	}
	if body["garmentTargets"].(map[string]any)["top_chest"].(map[string]any)["min"].(float64) != 113 {
		t.Fatal("changed clothing target")
	}
	w = request(a, "PUT", "/api/body/BODY", `{"version":1,"measurements":{"chest":93}}`)
	if w.Code != 409 {
		t.Fatal("stale body update accepted", w.Code, w.Body.String())
	}
	w = request(a, "PUT", "/api/body/BODY", `{"version":2,"measurements":{"unknown":90}}`)
	if w.Code != 400 {
		t.Fatal("unrecognized body field accepted", w.Code)
	}
	w = request(a, "GET", "/api/hairstyles", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "HAIR0001") {
		t.Fatal("hairstyle missing", w.Body.String())
	}
	w = request(a, "GET", "/api/export", "")
	var all map[string]any
	if e = json.Unmarshal(w.Body.Bytes(), &all); e != nil {
		t.Fatal(e)
	}
	if _, ok := all["body"]; !ok {
		t.Fatal("body missing from export")
	}
	if _, ok := all["hairstyles"]; !ok {
		t.Fatal("hairstyles missing from export")
	}
}
