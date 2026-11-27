package main

import (
	"errors"
	"testing"
)

func TestProductClassificationRegression(t *testing.T) {
	for _, name := range []string{"京东京造背心式垃圾袋45*60cm", "手机钢化膜送眼镜布", "【赠送·点击查看】60天质量换新-JC-JZ服饰箱包眼镜0-100", "酒精干湿包", "厨房清洁手套"} {
		if cat, ok := classifyOrder(name, "黑色 L"); ok {
			t.Fatalf("nonwearable %s: %s", name, cat)
		}
	}
	for _, name := range []string{"京东京造【超速干套装】跑步运动套装吸湿速干T恤五分短裤男秋", "京东京造【呼吸套装】跑步运动套装"} {
		if cat, ok := classifyOrder(name, ""); !ok || cat != "套装" {
			t.Fatalf("set %s: %s", name, cat)
		}
	}
}
func TestProductIdentityAcrossPurchases(t *testing.T) {
	a := testApp(t)
	input := OrderImport{Platform: "test", Items: []OrderItem{
		{OrderID: "A", LineID: "1", Name: "男士运动裤", Spec: "黑色 L", URL: "https://shop.example/item?id=1"},
		{OrderID: "B", LineID: "2", Name: "男士运动裤", Spec: "黑色·L", URL: "https://shop.example/item?id=1&utm_source=test"},
		{OrderID: "C", LineID: "3", Name: "男士运动裤", Spec: "黑色 XL", URL: "https://shop.example/item?id=1"},
		{OrderID: "D", LineID: "4", Name: "男士运动裤", URL: "https://shop.example/item?id=1"},
	}}
	if _, e := a.s.importOrders(input); e != nil {
		t.Fatal(e)
	}
	promote := func(id string) (record, error) {
		return a.s.promoteOrderItem("test:" + id + ":" + map[string]string{"A": "1", "B": "2", "C": "3", "D": "4"}[id])
	}
	x, e := promote("A")
	if e != nil {
		t.Fatal(e)
	}
	y, e := promote("B")
	if e != nil || x["id"] != y["id"] {
		t.Fatal("duplicate", e)
	}
	z, e := promote("C")
	if e != nil || x["id"] == z["id"] {
		t.Fatal("variant", e)
	}
	if _, e = promote("D"); !errors.Is(e, errProductReview) {
		t.Fatal("unknown spec should require review", e)
	}
}
func TestArchiveHistoricalNonwearableAndDuplicate(t *testing.T) {
	a := testApp(t)
	legacy, err := a.s.save("products", "LEGACY", record{"name": "男士运动裤", "category": "裤装", "spec": "黑色 L", "url": "https://shop.example/item/1", "rating": 4}, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = legacy
	in := OrderImport{Platform: "test", Items: []OrderItem{
		{OrderID: "A", LineID: "1", Name: "男士运动裤", Spec: "黑色 L", URL: "https://shop.example/item/1"},
		{OrderID: "B", LineID: "2", Name: "背心式垃圾袋", URL: "https://shop.example/item/2"},
	}}
	if _, err = a.s.importOrders(in); err != nil {
		t.Fatal(err)
	}
	for n, i := range in.Items {
		key := "test:" + i.OrderID + ":" + i.LineID
		cat := "裤装"
		if n == 1 {
			cat = "家居与保暖内搭"
		}
		p, e := a.s.importProduct(CreateProductInput{SourceKey: key, Platform: "test", OrderID: i.OrderID, Verification: "verified", Name: i.Name, Category: cat, Spec: i.Spec, URL: i.URL})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = a.s.db.Exec("UPDATE order_items SET product_id=? WHERE source_key=?", p["id"], key); e != nil {
			t.Fatal(e)
		}
	}
	plan, e := a.s.repairWardrobe("")
	if e != nil || len(plan.Actions) != 2 {
		t.Fatalf("plan %+v %v", plan, e)
	}
	if _, e = a.s.repairWardrobe("stale"); e == nil {
		t.Fatal("stale confirmation accepted")
	}
	applied, e := a.s.repairWardrobe(plan.Token)
	if e != nil || applied.Applied != 2 {
		t.Fatalf("apply %+v %v", applied, e)
	}
	rows, e := a.s.orders()
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range rows {
		if i.OrderID == "A" && i.ProductID != "LEGACY" {
			t.Fatal("link lost")
		}
		if i.OrderID == "B" && (i.ProductID != "" || i.Wearable) {
			t.Fatal("trash retained")
		}
	}
	var n int
	if e = a.s.db.QueryRow("SELECT count(*) FROM wardrobe_product_archive").Scan(&n); e != nil || n != 2 {
		t.Fatal("archive", n, e)
	}
	again, e := a.s.repairWardrobe("")
	if e != nil || len(again.Actions) != 0 {
		t.Fatal("not idempotent", again, e)
	}
	if got, e := a.s.get("products", "LEGACY"); e != nil || got["rating"] != float64(4) {
		t.Fatal("legacy rating lost", e)
	}
}

func TestRepairKeepsOutfitProductAndMergesUnreferencedDuplicate(t *testing.T) {
	a := testApp(t)
	input := OrderImport{Platform: "test", Items: []OrderItem{
		{OrderID: "A", LineID: "1", Name: "男士运动裤", Spec: "黑色 L", URL: "https://shop.example/item/9"},
		{OrderID: "B", LineID: "2", Name: "男士运动裤", Spec: "黑色 L", URL: "https://shop.example/item/9"},
	}}
	if _, e := a.s.importOrders(input); e != nil {
		t.Fatal(e)
	}
	ids := []string{}
	for _, i := range input.Items {
		key := "test:" + i.OrderID + ":" + i.LineID
		p, e := a.s.importProduct(CreateProductInput{SourceKey: key, Platform: "test", OrderID: i.OrderID, Verification: "verified", Name: i.Name, Category: "裤装", Spec: i.Spec, URL: i.URL})
		if e != nil {
			t.Fatal(e)
		}
		id := productText(p["id"])
		ids = append(ids, id)
		if _, e = a.s.db.Exec("UPDATE order_items SET product_id=? WHERE source_key=?", id, key); e != nil {
			t.Fatal(e)
		}
	}
	// Protect the lexically later record so ordering actually matters.
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}
	look, e := a.s.save("looks", "LOOK-REPAIR", record{"name": "test", "slots": map[string]any{"pants": ids[1]}}, 0)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := a.s.repairWardrobe("")
	if e != nil || len(plan.Actions) != 1 || plan.Actions[0].ID != ids[0] || plan.Actions[0].Target != ids[1] {
		t.Fatalf("plan %+v %v", plan, e)
	}
	if _, e = a.s.repairWardrobe(plan.Token); e != nil {
		t.Fatal(e)
	}
	got, e := a.s.get("looks", "LOOK-REPAIR")
	if e != nil || got["version"] != look["version"] || got["slots"].(map[string]any)["pants"] != ids[1] {
		t.Fatal("outfit changed", e)
	}
	if _, e = a.s.get("products", ids[1]); e != nil {
		t.Fatal("referenced item lost", e)
	}
}
