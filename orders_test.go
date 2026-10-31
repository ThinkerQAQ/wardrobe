package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

func syntheticOrder() OrderImport {
	return OrderImport{Platform: "淘宝", Items: []OrderItem{
		{OrderID: "FAKE-TB-100", LineID: "line1", Name: "男士真皮半指手套", Spec: "黑色 L码", Quantity: 1, OrderStatus: "交易成功，退款成功", RefundStatus: "退款成功", OrderedAt: "2026-10-25 18:00:00", Amount: "99.00", Paid: "79.00"},
		{OrderID: "FAKE-TB-101", Name: "洗衣液补充装", Quantity: 2, OrderedAt: "2026-10-22 15:00:00"},
	}}
}
func TestUnifiedOrdersNoDuplicatesAndManualDecisions(t *testing.T) {
	a := testApp(t)
	initial, e := a.s.importOrders(syntheticOrder())
	if e != nil {
		t.Fatal(e)
	}
	if initial.Inserted != 2 || initial.Wearable != 1 {
		t.Fatalf("unexpected import %+v", initial)
	}
	rows, e := a.s.orders()
	if e != nil || len(rows) != 2 {
		t.Fatal(e, rows)
	}
	var glove OrderItem
	for _, r := range rows {
		if strings.Contains(r.Name, "手套") {
			glove = r
		} else if r.Wearable {
			t.Fatal("household item classified wearable")
		}
	}
	if glove.Owned != "unknown" || glove.Category != "配饰" {
		t.Fatalf("unexpected ownership/classification %+v", glove)
	}
	// A refund never means a returned garment; user's confirmation survives upsert.
	decision, e := a.s.updateOrderDecision(&glove.SourceKey, nil, ptr("owned"))
	if e != nil || decision.Owned != "owned" {
		t.Fatal(e, decision)
	}
	updated, e := a.s.importOrders(syntheticOrder())
	if e != nil || updated.Updated != 2 || updated.Inserted != 0 {
		t.Fatal(e, updated)
	}
	rows, _ = a.s.orders()
	for _, r := range rows {
		if r.SourceKey == glove.SourceKey && r.Owned != "owned" {
			t.Fatal("import overwrote explicit ownership")
		}
	}
	p, e := a.s.promoteOrderItem(glove.SourceKey)
	if e != nil {
		t.Fatal(e)
	}
	if p["category"] != "配饰" || p["owned"] != "本人确认持有" {
		t.Fatal("wardrobe product mismatch", p["category"], p["owned"])
	}
	same, e := a.s.promoteOrderItem(glove.SourceKey)
	if e != nil || same["id"] != p["id"] {
		t.Fatal("duplicate promotion", e)
	}
	all, e := a.s.list("products")
	if e != nil || len(all) != 2 {
		t.Fatal("expected only 1 new wardrobe item")
	}
}
func ptr(s string) *string { return &s }
func TestOrdersManualCategorySurvivesImport(t *testing.T) {
	a := testApp(t)
	_, e := a.s.importOrders(syntheticOrder())
	if e != nil {
		t.Fatal(e)
	}
	rows, _ := a.s.orders()
	var key string
	for _, r := range rows {
		if !r.Wearable {
			key = r.SourceKey
		}
	}
	v, e := a.s.updateOrderDecision(&key, ptr("配饰"), nil)
	if e != nil || !v.Wearable || !v.CategoryManual {
		t.Fatal(e, v)
	}
	_, e = a.s.importOrders(syntheticOrder())
	if e != nil {
		t.Fatal(e)
	}
	rows, _ = a.s.orders()
	for _, r := range rows {
		if r.SourceKey == key && (!r.Wearable || r.Category != "配饰") {
			t.Fatal("manual category reset")
		}
	}
}
func TestCSVAndXLSXImport(t *testing.T) {
	csv := []byte("\xef\xbb\xbf订单号,订单提交时间,商品名称,型号款式,商品数量,订单状态,实付金额\nFAKE-1,2026-10-24 10:00:00,皮鞋,黑色42码,2,交易成功,190.00\n")
	parsed, e := readOrderFile("淘宝", "orders.csv", csv)
	if e != nil || len(parsed.Items) != 1 || parsed.Items[0].Quantity != 2 {
		t.Fatalf("csv: %+v %v", parsed, e)
	}
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	file, _ := zw.Create("xl/worksheets/sheet1.xml")
	row := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>订单号</t></is></c><c r="B1" t="inlineStr"><is><t>商品名称</t></is></c><c r="C1" t="inlineStr"><is><t>型号款式</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>FAKE-2</t></is></c><c r="B2" t="inlineStr"><is><t>切尔西靴</t></is></c><c r="C2" t="inlineStr"><is><t>42码</t></is></c></row></sheetData></worksheet>`
	file.Write([]byte(row))
	zw.Close()
	out, e := readOrderFile("京东", "test.xlsx", buf.Bytes())
	if e != nil || len(out.Items) != 1 || out.Items[0].Name != "切尔西靴" {
		t.Fatalf("xlsx: %+v %v", out, e)
	}
	// Fail closed on malformed/oversized archives; never execute macros or formulas.
	if _, e = readOrderFile("京东", "test.xlsx", []byte("fake")); e == nil {
		t.Fatal("invalid XLSX accepted")
	}
	if _, e = readOrderFile("京东", "test.xls", csv); e == nil {
		t.Fatal("unsupported legacy .xls accepted")
	}
	var decoded orderXSheet
	if e = xml.Unmarshal([]byte(row), &decoded); e != nil || len(decoded.Rows) != 2 {
		t.Fatal(e)
	}
}
func TestOrderFileUploadAndPairingSecurity(t *testing.T) {
	a := testApp(t)
	b := new(bytes.Buffer)
	form := multipart.NewWriter(b)
	form.WriteField("platform", "淘宝")
	f, _ := form.CreateFormFile("file", "orders.csv")
	f.Write([]byte("订单号,商品名称,商品数量\nFAKE-44,真皮手套,1\n"))
	form.Close()
	r := httptest.NewRequest("POST", "http://127.0.0.1:17936/api/orders/import-file", bytes.NewReader(b.Bytes()))
	r.Header.Set("Content-Type", form.FormDataContentType())
	w := httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(a, "GET", "/api/orders", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "真皮手套") {
		t.Fatal("order list unavailable")
	}
	key, e := a.s.orderImportKey()
	if e != nil || len(key) != 64 {
		t.Fatal("pairing key invalid", e)
	}
	extOrigin := "chrome-extension://" + strings.Repeat("a", 32)
	payload, _ := json.Marshal(OrderImport{Platform: "京东", Items: []OrderItem{{OrderID: "FAKE-55", Name: "牛仔裤"}}})
	post := func(origin, k string) int {
		r := httptest.NewRequest("POST", "http://127.0.0.1:17936/api/orders/ingest", bytes.NewReader(payload))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Wardrobe-Import-Key", k)
		w := httptest.NewRecorder()
		a.ServeHTTP(w, r)
		return w.Code
	}
	if post("https://evil.example", key) != 403 {
		t.Fatal("cross-site fake origin accepted")
	}
	if post(extOrigin, "bad") != 403 {
		t.Fatal("bad pairing key accepted")
	}
	if post(extOrigin, key) != 200 {
		t.Fatal("authorized import failed")
	}
	r = httptest.NewRequest("GET", "http://127.0.0.1:17936/api/orders/pairing", nil)
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	a.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("pairing key exposed cross-site")
	}
}

func TestOrdersIncludedInLocalJSONBackup(t *testing.T) {
	a := testApp(t)
	if _, e := a.s.importOrders(syntheticOrder()); e != nil {
		t.Fatal(e)
	}
	w := request(a, "GET", "/api/export", "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var body map[string]json.RawMessage
	if e := json.Unmarshal(w.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	var items []OrderItem
	if e := json.Unmarshal(body["orders"], &items); e != nil || len(items) != 2 {
		t.Fatal("order backup missing", e)
	}
}

func TestOwnershipCorrectionAfterPromotionUpdatesInventory(t *testing.T) {
	a := testApp(t)
	_, e := a.s.importOrders(syntheticOrder())
	if e != nil {
		t.Fatal(e)
	}
	rows, _ := a.s.orders()
	var key string
	for _, row := range rows {
		if row.Wearable {
			key = row.SourceKey
		}
	}
	product, e := a.s.promoteOrderItem(key)
	if e != nil {
		t.Fatal(e)
	}
	if product["owned"] != "待确认" {
		t.Fatal("unverified item must stay pending")
	}
	_, e = a.s.updateOrderDecision(&key, nil, ptr("owned"))
	if e != nil {
		t.Fatal(e)
	}
	product, e = a.s.get("products", product["id"].(string))
	if e != nil || product["owned"] != "本人确认持有" {
		t.Fatal("linked inventory not updated", e)
	}
	_, e = a.s.updateOrderDecision(&key, nil, ptr("returned"))
	if e != nil {
		t.Fatal(e)
	}
	product, e = a.s.get("products", product["id"].(string))
	if e != nil || product["owned"] != "已退货" {
		t.Fatal("return status not synced", e)
	}
}
