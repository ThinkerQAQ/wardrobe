package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrganizeWearablesIntoPendingWardrobeWithoutDuplicates(t *testing.T) {
	a := testApp(t)
	src := OrderImport{Platform: "京东", Items: []OrderItem{
		{OrderID: "TEST-JD-01", LineID: "1", Name: "男士切尔西靴", Quantity: 1, ImageURL: "https://assets.example.invalid/test.jpg", RefundStatus: "已退款"},
		{OrderID: "TEST-JD-02", LineID: "2", Name: "男士皮手套", Quantity: 1},
		{OrderID: "TEST-JD-03", LineID: "3", Name: "洗衣液", Quantity: 1},
		{OrderID: "TEST-JD-04", LineID: "4", Name: "男士运动鞋", Quantity: 1},
	}}
	if _, e := a.s.importOrders(src); e != nil {
		t.Fatal(e)
	}
	rows, _ := a.s.orders()
	for _, v := range rows {
		if strings.Contains(v.Name, "运动鞋") {
			_, e := a.s.updateOrderDecision(&v.SourceKey, nil, ptr("returned"))
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	first, e := a.s.organizeWearables(1)
	if e != nil || first.Created != 1 || first.Remaining != 1 {
		t.Fatalf("first batch: %+v %v", first, e)
	}
	second, e := a.s.organizeWearables(10)
	if e != nil || second.Created != 1 || second.Remaining != 0 {
		t.Fatalf("second batch: %+v %v", second, e)
	}
	third, e := a.s.organizeWearables(10)
	if e != nil || third.Created != 0 || third.Remaining != 0 {
		t.Fatalf("idempotence: %+v %v", third, e)
	}
	products, _ := a.s.list("products")
	if len(products) != 3 {
		t.Fatalf("expected seed and 2 candidates; got %d", len(products))
	}
	for _, p := range products {
		if p["id"] == "W0001" {
			continue
		}
		if p["owned"] != "待确认" || p["rating"] != float64(0) {
			t.Fatal("auto-import manufactured ownership/rating")
		}
	}
	got, _ := a.s.orders()
	for _, v := range got {
		if (strings.Contains(v.Name, "运动鞋") || strings.Contains(v.Name, "洗衣液")) && v.ProductID != "" {
			t.Fatal("excluded item entered wardrobe")
		}
		if strings.Contains(v.Name, "切尔西靴") && v.ProductID == "" {
			t.Fatal("refunded but not returned item was ignored")
		}
	}
	if _, e = a.s.organizeWearables(1000); e == nil {
		t.Fatal("unbounded batch accepted")
	}
}

func TestOrderOrganizeAPIAndImageEndpointValidation(t *testing.T) {
	a := testApp(t)
	w := request(a, "POST", "/api/orders/organize", `{"limit":10}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result OrderOrganizeResult
	if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
		t.Fatal(e)
	}
	for _, v := range []string{`{"limit":9999}`, `{"limit":-3}`, `{"limit":5,"arbitrary":"X"}`} {
		if got := request(a, "POST", "/api/orders/organize", v); got.Code != 400 {
			t.Fatal("invalid batch accepted", v)
		}
	}
	if got := request(a, "POST", "/api/orders/cache-images", `{"limit":11}`); got.Code != 400 {
		t.Fatal("oversized image batch accepted")
	}
	if got := request(a, "POST", "/api/orders/cache-images", `{"limit":1}`); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
}

func TestNewAccessoriesAreClassifiedAndManualDecisionsPreserved(t *testing.T) {
	a := testApp(t)
	input := OrderImport{Platform: "淘宝", Items: []OrderItem{
		{OrderID: "FAKE-ACC-1", LineID: "A", Name: "复古金属胸针", Quantity: 1},
		{OrderID: "FAKE-ACC-2", LineID: "B", Name: "男士袖扣礼盒", Quantity: 1},
		{OrderID: "FAKE-ACC-3", LineID: "C", Name: "发夹套装", Quantity: 1},
		{OrderID: "FAKE-ACC-4", LineID: "D", Name: "手镯编织款", Quantity: 1},
		{OrderID: "FAKE-ACC-5", LineID: "E", Name: "衣服收纳袋", Quantity: 1},
	}}
	if _, e := a.s.importOrders(input); e != nil {
		t.Fatal(e)
	}
	rows, _ := a.s.orders()
	for _, v := range rows {
		if v.Name == "衣服收纳袋" {
			if v.Wearable {
				t.Fatal("non-garment accessory")
			}
			continue
		}
		if !v.Wearable || v.Category != "配饰" {
			t.Fatalf("missing wearable accessory: %s", v.Name)
		}
		if v.Name == "男士袖扣礼盒" {
			cat := ""
			if _, e := a.s.updateOrderDecision(&v.SourceKey, &cat, nil); e != nil {
				t.Fatal(e)
			}
		}
	}
	if _, e := a.s.organizeWearables(30); e != nil {
		t.Fatal(e)
	}
	rows, _ = a.s.orders()
	for _, v := range rows {
		if v.Name == "男士袖扣礼盒" && (v.Wearable || v.ProductID != "") {
			t.Fatal("manual category override lost")
		}
	}
}
