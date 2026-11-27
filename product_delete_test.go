package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestBatchDeleteProtectsOutfitAndPreventsReimport(t *testing.T) {
	a := testApp(t)
	in := OrderImport{Platform: "test", Items: []OrderItem{{OrderID: "A", LineID: "1", Name: "男士运动裤", Spec: "L"}, {OrderID: "B", LineID: "1", Name: "男士运动鞋", Spec: "42"}}}
	if _, e := a.s.importOrders(in); e != nil {
		t.Fatal(e)
	}
	x, e := a.s.promoteOrderItem("test:A:1")
	if e != nil {
		t.Fatal(e)
	}
	y, e := a.s.promoteOrderItem("test:B:1")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.s.save("looks", "LOOK-PROTECT", record{"slots": map[string]any{"pants": x["id"]}}, 0); e != nil {
		t.Fatal(e)
	}
	out, e := a.s.deleteWardrobeProducts([]ProductDeleteSelection{{productText(x["id"]), x["version"].(int)}, {productText(y["id"]), y["version"].(int)}})
	if e != nil || out.Deleted != 1 || len(out.Protected) != 1 {
		t.Fatal(out, e)
	}
	if _, e = a.s.importOrders(in); e != nil {
		t.Fatal(e)
	}
	rows, e := a.s.orders()
	if e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		if r.OrderID == "B" && (r.Wearable || !r.CategoryManual || r.ProductID != "") {
			t.Fatal("deleted product can reappear")
		}
	}
	if _, e = a.s.get("products", productText(x["id"])); e != nil {
		t.Fatal(e)
	}
}
func TestWardrobeImageReusesLocalThumbnail(t *testing.T) {
	a := testApp(t)
	in := OrderImport{Platform: "test", Items: []OrderItem{{OrderID: "A", LineID: "1", Name: "男士运动裤", Spec: "L", ImageURL: "https://invalid.example/thumbnail.jpg"}}}
	if _, e := a.s.importOrders(in); e != nil {
		t.Fatal(e)
	}
	p, e := a.s.promoteOrderItem("test:A:1")
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 120, 120)), nil); e != nil {
		t.Fatal(e)
	}
	path := a.s.orderThumbnailPath("test:A:1", in.Items[0].ImageURL)
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out, e := a.s.cacheWardrobeImages(context.Background(), "", 10, wardrobeImageHTTPClient)
	if e != nil || out.Cached != 1 {
		t.Fatal(out, e)
	}
	p, e = a.s.get("products", productText(p["id"]))
	if e != nil || productText(p["image"]) == "" {
		t.Fatal("image missing", e)
	}
}
