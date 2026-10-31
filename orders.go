package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Purchase data is separate from the confirmed wardrobe inventory. Never persist
// an unfiltered platform response: it can contain addresses, phone numbers or credentials.
type OrderItem struct {
	SourceKey      string `json:"source_key"`
	Platform       string `json:"platform"`
	OrderID        string `json:"order_id"`
	LineID         string `json:"line_id,omitempty"`
	OrderedAt      string `json:"ordered_at,omitempty"`
	Shop           string `json:"shop,omitempty"`
	Name           string `json:"name"`
	Spec           string `json:"spec,omitempty"`
	URL            string `json:"url,omitempty"`
	ImageURL       string `json:"image_url,omitempty"`
	LocalImage     string `json:"local_image,omitempty"`
	Quantity       int    `json:"quantity"`
	Amount         string `json:"amount,omitempty"`
	Paid           string `json:"paid,omitempty"`
	OrderStatus    string `json:"order_status,omitempty"`
	RefundStatus   string `json:"refund_status,omitempty"`
	ReturnStatus   string `json:"return_status,omitempty"`
	Category       string `json:"category,omitempty"`
	Wearable       bool   `json:"wearable"`
	CategoryManual bool   `json:"category_manual"`
	Owned          string `json:"owned"` // unknown / owned / returned / not_owned
	ProductID      string `json:"product_id,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

type OrderImport struct {
	Platform string      `json:"platform"`
	Items    []OrderItem `json:"items"`
}
type OrderImportResult struct {
	Total    int `json:"total"`
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Wearable int `json:"wearable"`
}

func (s *store) ensureOrders() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS purchase_orders (
 platform TEXT NOT NULL, order_id TEXT NOT NULL, ordered_at TEXT NOT NULL DEFAULT '',shop TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL,
 PRIMARY KEY(platform,order_id));
 CREATE TABLE IF NOT EXISTS order_items (
 source_key TEXT PRIMARY KEY,platform TEXT NOT NULL,order_id TEXT NOT NULL,line_id TEXT NOT NULL DEFAULT '', ordered_at TEXT NOT NULL DEFAULT '',shop TEXT NOT NULL DEFAULT '',
 name TEXT NOT NULL,spec TEXT NOT NULL DEFAULT '',url TEXT NOT NULL DEFAULT '',image_url TEXT NOT NULL DEFAULT '',quantity INTEGER NOT NULL DEFAULT 1,
 amount TEXT NOT NULL DEFAULT '',paid TEXT NOT NULL DEFAULT '',order_status TEXT NOT NULL DEFAULT '',refund_status TEXT NOT NULL DEFAULT '',return_status TEXT NOT NULL DEFAULT '',
 category TEXT NOT NULL DEFAULT '',wearable INTEGER NOT NULL DEFAULT 0,category_manual INTEGER NOT NULL DEFAULT 0,owned TEXT NOT NULL DEFAULT 'unknown',product_id TEXT NOT NULL DEFAULT '',updated_at TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS idx_order_items_platform_date ON order_items(platform,ordered_at);
 CREATE INDEX IF NOT EXISTS idx_order_items_platform_order ON order_items(platform,order_id,line_id);
 CREATE INDEX IF NOT EXISTS idx_order_items_wearable ON order_items(wearable,category);`)
	return err
}

var nonWardrobeKeywords = []string{"鞋柜", "洗衣液", "鞋刷", "衣架", "晾衣", "洗鞋", "擦鞋", "鞋油", "衣物收纳", "服装收纳", "洗衣机", "衣服清洁", "衣物护理", "购物袋", "服装袋", "服饰专用洗", "鞋盒", "衣服架", "改衣", "衣柜", "衣服防尘罩", "衣物柔顺", "洗发露", "洗发水"}
var garmentCategories = []struct {
	Category string
	Keywords []string
}{
	{"配饰", []string{"手套", "围巾", "丝巾", "领带", "领结", "眼镜", "墨镜", "太阳镜", "皮带", "腰带", "手表", "腕表", "帽子", "耳饰", "项链", "戒指", "手链", "手镯", "手串", "腕带", "耳环", "耳钉", "胸针", "袖扣", "发夹", "发带", "发箍", "发饰", "吊坠", "脚链", "口袋巾", "领针", "挎包", "背包", "手提包", "钱包", "斜挎包", "公文包"}},
	{"袜子", []string{"袜子", "船袜", "丝袜", "短袜", "长袜"}},
	{"鞋靴", []string{"皮鞋", "靴子", "切尔西", "马丁靴", "运动鞋", "跑鞋", "帆布鞋", "休闲鞋", "高帮鞋", "低帮鞋", "雪地靴", "凉鞋", "拖鞋", "球鞋", "板鞋", "乐福鞋", "布洛克鞋", "德比鞋", "牛津鞋", "鞋靴"}},
	{"裤装", []string{"牛仔裤", "西裤", "工装裤", "运动裤", "休闲裤", "短裤", "长裤", "阔腿裤", "直筒裤", "裤子", "七分裤", "九分裤"}},
	{"外套", []string{"外套", "大衣", "风衣", "羽绒服", "夹克", "西装", "西服", "棉服", "冲锋衣", "皮衣", "棒球服", "毛呢"}},
	{"衬衫", []string{"衬衫", "衬衣"}},
	{"针织衫", []string{"毛衣", "针织衫", "开衫"}},
	{"卫衣", []string{"卫衣"}},
	{"T恤与POLO", []string{"t恤", "t-shirt", "t shirt", "polo衫", "polo 衫", "短袖上衣", "长袖上衣"}},
	{"家居与保暖内搭", []string{"内衣", "保暖衣", "秋衣", "秋裤", "背心", "睡衣", "睡袍", "打底衫", "打底裤"}},
}

func classifyOrder(name, spec string) (string, bool) {
	s := strings.ToLower(name + " " + spec)
	for _, bad := range nonWardrobeKeywords {
		if strings.Contains(s, bad) {
			return "", false
		}
	}
	for _, group := range garmentCategories {
		for _, word := range group.Keywords {
			if strings.Contains(s, word) {
				return group.Category, true
			}
		}
	}
	return "", false
}
func validPlatform(p string) bool {
	return len(p) > 0 && len(p) <= 40 && strings.TrimSpace(p) == p && !strings.ContainsAny(p, "\r\n\t:")
}
func sourceIdentity(i OrderItem) string {
	if i.LineID != "" {
		return i.Platform + ":" + i.OrderID + ":" + i.LineID
	}
	x := i.Platform + "\x00" + i.OrderID + "\x00" + strings.ToLower(strings.TrimSpace(i.Name)) + "\x00" + strings.ToLower(strings.TrimSpace(i.Spec)) + "\x00" + strings.TrimSpace(i.URL)
	h := sha256.Sum256([]byte(x))
	return strings.ToLower(i.Platform) + ":" + i.OrderID + ":" + hex.EncodeToString(h[:12])
}
func cleanOrderItem(i OrderItem, platform string) (OrderItem, error) {
	i.Platform = platform
	i.OrderID = strings.TrimSpace(i.OrderID)
	i.LineID = strings.TrimSpace(i.LineID)
	i.Name = strings.TrimSpace(i.Name)
	if !validPlatform(platform) || i.OrderID == "" || i.Name == "" || len(i.OrderID) > 100 || len(i.LineID) > 120 || len(i.Name) > 500 {
		return i, errors.New("platform, real order ID and item name required")
	}
	for _, x := range []string{i.OrderedAt, i.Shop, i.Spec, i.URL, i.ImageURL, i.Amount, i.Paid, i.OrderStatus, i.RefundStatus, i.ReturnStatus} {
		if len(x) > 1500 {
			return i, errors.New("order field too long")
		}
	}
	for _, u := range []string{i.URL, i.ImageURL} {
		if u != "" && !(strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")) {
			return i, errors.New("invalid item URL")
		}
	}
	if i.Quantity == 0 {
		i.Quantity = 1
	}
	if i.Quantity < 1 || i.Quantity > 10000 {
		return i, errors.New("invalid quantity")
	}
	i.SourceKey = sourceIdentity(i)
	i.Category, i.Wearable = classifyOrder(i.Name, i.Spec)
	i.Owned = "unknown"
	i.ProductID = ""
	i.CategoryManual = false
	return i, nil
}

func (s *store) importOrders(input OrderImport) (OrderImportResult, error) {
	result := OrderImportResult{Total: len(input.Items)}
	if !validPlatform(input.Platform) || len(input.Items) == 0 || len(input.Items) > 20000 {
		return result, errors.New("expected 1..20000 items from a valid order source")
	}
	tx, e := s.db.Begin()
	if e != nil {
		return result, e
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	incomingCounts := map[string]int{}
	for _, raw := range input.Items {
		raw.Platform = input.Platform
		if key := privateIncomingOrderKey(raw); key != "" {
			incomingCounts[key]++
		}
	}
	for _, item := range input.Items {
		i, err := cleanOrderItem(item, input.Platform)
		if err != nil {
			return result, err
		}
		legacyKey, legacySpec, err := privateMatchOrder(s, tx, i, incomingCounts[privateIncomingOrderKey(i)] == 1)
		if err != nil {
			return result, err
		}
		if legacyKey != "" {
			i.SourceKey = legacyKey
			if legacySpec != "" {
				i.Spec = legacySpec
				i.Category, i.Wearable = classifyOrder(i.Name, i.Spec)
			}
		}
		var old OrderItem
		var oldManual, oldWearable int
		err = tx.QueryRow(`SELECT category,wearable,category_manual,owned,product_id FROM order_items WHERE source_key=?`, i.SourceKey).Scan(&old.Category, &oldWearable, &oldManual, &old.Owned, &old.ProductID)
		if err != nil && err != sql.ErrNoRows {
			return result, err
		}
		isNew := err == sql.ErrNoRows
		if !isNew {
			i.Owned = old.Owned
			i.ProductID = old.ProductID
			i.CategoryManual = oldManual != 0
			if i.CategoryManual {
				i.Category = old.Category
				i.Wearable = oldWearable == 1
			}
		}
		_, err = tx.Exec(`INSERT INTO purchase_orders(platform,order_id,ordered_at,shop,status,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(platform,order_id) DO UPDATE SET ordered_at=CASE WHEN excluded.ordered_at<>'' THEN excluded.ordered_at ELSE purchase_orders.ordered_at END,shop=CASE WHEN excluded.shop<>'' THEN excluded.shop ELSE purchase_orders.shop END,status=CASE WHEN excluded.status<>'' THEN excluded.status ELSE purchase_orders.status END,updated_at=excluded.updated_at`, i.Platform, i.OrderID, i.OrderedAt, i.Shop, i.OrderStatus, now)
		if err != nil {
			return result, err
		}
		_, err = tx.Exec(`INSERT INTO order_items(source_key,platform,order_id,line_id,ordered_at,shop,name,spec,url,image_url,quantity,amount,paid,order_status,refund_status,return_status,category,wearable,category_manual,owned,product_id,updated_at)
   VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
   ON CONFLICT(source_key) DO UPDATE SET ordered_at=CASE WHEN excluded.ordered_at<>'' THEN excluded.ordered_at ELSE order_items.ordered_at END,shop=CASE WHEN excluded.shop<>'' THEN excluded.shop ELSE order_items.shop END,name=excluded.name,spec=excluded.spec,url=CASE WHEN excluded.url<>'' THEN excluded.url ELSE order_items.url END,image_url=CASE WHEN excluded.image_url<>'' THEN excluded.image_url ELSE order_items.image_url END,quantity=excluded.quantity,amount=CASE WHEN excluded.amount<>'' THEN excluded.amount ELSE order_items.amount END,paid=CASE WHEN excluded.paid<>'' THEN excluded.paid ELSE order_items.paid END,order_status=CASE WHEN excluded.order_status<>'' THEN excluded.order_status ELSE order_items.order_status END,refund_status=CASE WHEN excluded.refund_status<>'' THEN excluded.refund_status ELSE order_items.refund_status END,return_status=CASE WHEN excluded.return_status<>'' THEN excluded.return_status ELSE order_items.return_status END,category=CASE WHEN order_items.category_manual=1 THEN order_items.category ELSE excluded.category END,wearable=CASE WHEN order_items.category_manual=1 THEN order_items.wearable ELSE excluded.wearable END,updated_at=excluded.updated_at`,
			i.SourceKey, i.Platform, i.OrderID, i.LineID, i.OrderedAt, i.Shop, i.Name, i.Spec, i.URL, i.ImageURL, i.Quantity, i.Amount, i.Paid, i.OrderStatus, i.RefundStatus, i.ReturnStatus, i.Category, boolInt(i.Wearable), boolInt(i.CategoryManual), i.Owned, i.ProductID, now)
		if err != nil {
			return result, err
		}
		if isNew {
			result.Inserted++
		} else {
			result.Updated++
		}
		if i.Wearable {
			result.Wearable++
		}
	}
	return result, tx.Commit()
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func (s *store) orders() ([]OrderItem, error) {
	rows, e := s.db.Query(`SELECT source_key,platform,order_id,line_id,ordered_at,shop,name,spec,url,image_url,quantity,amount,paid,order_status,refund_status,return_status,category,wearable,category_manual,owned,product_id,updated_at FROM order_items ORDER BY ordered_at DESC, source_key DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []OrderItem{}
	for rows.Next() {
		var x OrderItem
		var w, m int
		if e = rows.Scan(&x.SourceKey, &x.Platform, &x.OrderID, &x.LineID, &x.OrderedAt, &x.Shop, &x.Name, &x.Spec, &x.URL, &x.ImageURL, &x.Quantity, &x.Amount, &x.Paid, &x.OrderStatus, &x.RefundStatus, &x.ReturnStatus, &x.Category, &w, &m, &x.Owned, &x.ProductID, &x.UpdatedAt); e != nil {
			return nil, e
		}
		x.Wearable = w == 1
		x.CategoryManual = m == 1
		if x.ProductID != "" && validID.MatchString(x.ProductID) {
			if _, err := os.Stat(filepath.Join(s.dir, "images", x.ProductID+".jpg")); err == nil {
				x.LocalImage = "/media/images/" + x.ProductID + ".jpg"
			}
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *store) updateOrderDecision(key, category, owned *string) (OrderItem, error) {
	if key == nil || *key == "" {
		return OrderItem{}, errors.New("source_key required")
	}
	if category == nil && owned == nil {
		return OrderItem{}, errors.New("nothing to change")
	}
	tx, e := s.db.Begin()
	if e != nil {
		return OrderItem{}, e
	}
	defer tx.Rollback()
	var oldCategory, oldOwned, productID string
	var w, m int
	if e = tx.QueryRow(`SELECT category,owned,wearable,category_manual,product_id FROM order_items WHERE source_key=?`, *key).Scan(&oldCategory, &oldOwned, &w, &m, &productID); e != nil {
		return OrderItem{}, e
	}
	if category != nil {
		ok := *category == ""
		for _, g := range garmentCategories {
			ok = ok || g.Category == *category
		}
		if !ok {
			return OrderItem{}, errors.New("invalid category")
		}
		oldCategory = *category
		w = boolInt(*category != "")
		m = 1
	}
	if owned != nil {
		switch *owned {
		case "unknown", "owned", "returned", "not_owned":
			oldOwned = *owned
		default:
			return OrderItem{}, errors.New("invalid owned status")
		}
	}
	_, e = tx.Exec(`UPDATE order_items SET category=?,wearable=?,category_manual=?,owned=?,updated_at=? WHERE source_key=?`, oldCategory, w, m, oldOwned, time.Now().UTC().Format(time.RFC3339), *key)
	if e != nil {
		return OrderItem{}, e
	}
	if owned != nil && productID != "" {
		var body string
		var v int
		if e = tx.QueryRow(`SELECT body,version FROM records WHERE kind='products' AND id=?`, productID).Scan(&body, &v); e != nil {
			return OrderItem{}, e
		}
		var product record
		if e = json.Unmarshal([]byte(body), &product); e != nil {
			return OrderItem{}, e
		}
		statuses := map[string]string{"unknown": "待确认", "owned": "本人确认持有", "returned": "已退货", "not_owned": "未拥有"}
		product["owned"] = statuses[oldOwned]
		product["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
		var raw []byte
		raw, e = json.Marshal(product)
		if e != nil {
			return OrderItem{}, e
		}
		_, e = tx.Exec(`UPDATE records SET body=?,version=version+1 WHERE kind='products' AND id=? AND version=?`, string(raw), productID, v)
		if e != nil {
			return OrderItem{}, e
		}
	}
	if e = tx.Commit(); e != nil {
		return OrderItem{}, e
	}
	rows, e := s.orders()
	if e != nil {
		return OrderItem{}, e
	}
	for _, i := range rows {
		if i.SourceKey == *key {
			return i, nil
		}
	}
	return OrderItem{}, sql.ErrNoRows
}
func (s *store) promoteOrderItem(key string) (record, error) {
	if len(key) > 250 || key == "" {
		return nil, errors.New("invalid source key")
	}
	rows, e := s.orders()
	if e != nil {
		return nil, e
	}
	var i *OrderItem
	for idx := range rows {
		if rows[idx].SourceKey == key {
			i = &rows[idx]
			break
		}
	}
	if i == nil {
		return nil, sql.ErrNoRows
	}
	if !i.Wearable || i.Owned == "returned" || i.Owned == "not_owned" {
		return nil, errors.New("item is not eligible for wardrobe")
	}
	if i.ProductID != "" {
		return s.get("products", i.ProductID)
	}
	// Reuse only a unique exact purchase match; never guess by similar titles.
	prior, e := s.list("products")
	if e != nil {
		return nil, e
	}
	var match record
	ambiguous := false
	for _, p := range prior {
		fields, _ := p["fields"].(map[string]any)
		name, _ := p["name"].(string)
		spec, _ := p["spec"].(string)
		if fields["购买平台"] != i.Platform || fields["订单编号"] != i.OrderID || strings.TrimSpace(name) != i.Name {
			continue
		}
		if spec != "" && i.Spec != "" && spec != i.Spec {
			continue
		}
		if match != nil {
			ambiguous = true
			break
		}
		match = p
	}
	if match != nil && !ambiguous {
		if _, e = s.db.Exec("UPDATE order_items SET product_id=? WHERE source_key=?", match["id"], key); e != nil {
			return nil, e
		}
		return match, nil
	}
	// Existing source keys can be longer than the importer key limit or
	// contain Chinese characters; map them to a deterministic ASCII key.
	source := key
	if len(source) > 180 || !importKeyPattern.MatchString(source) {
		hash := sha256.Sum256([]byte(source))
		source = "order:" + hex.EncodeToString(hash[:16])
	}
	var existing string
	if err := s.db.QueryRow("SELECT product_id FROM product_import_keys WHERE source_key=?", source).Scan(&existing); err == nil {
		if _, e = s.db.Exec("UPDATE order_items SET product_id=? WHERE source_key=?", existing, key); e != nil {
			return nil, e
		}
		return s.get("products", existing)
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	fields := map[string]string{"订单状态": i.OrderStatus, "退款状态": i.RefundStatus, "退货状态": i.ReturnStatus, "店铺": i.Shop, "原订单商品键": key, "商品金额": i.Amount, "实付金额": i.Paid, "商品数量": strconv.Itoa(i.Quantity)}
	p, e := s.importProduct(CreateProductInput{SourceKey: source, Platform: i.Platform, OrderID: i.OrderID, Verification: "verified", Name: i.Name, Category: i.Category, Spec: i.Spec, Date: dateOnly(i.OrderedAt), URL: i.URL, Fields: fields})
	if e != nil {
		return nil, e
	}
	_, e = s.db.Exec("UPDATE order_items SET product_id=?,owned=CASE WHEN owned='owned' THEN 'owned' ELSE 'unknown' END WHERE source_key=?", p["id"], key)
	if e != nil {
		return nil, e
	}
	if i.Owned == "owned" {
		p["owned"] = "本人确认持有"
		p, e = s.save("products", p["id"].(string), p, p["version"].(int))
		if e != nil {
			return nil, e
		}
	}
	return p, nil
}
func dateOnly(s string) string {
	if len(s) >= 10 {
		v := s[:10]
		if _, e := time.Parse("2006-01-02", v); e == nil {
			return v
		}
	}
	return ""
}

func (a *application) serveOrders(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/orders":
		if r.Method != "GET" {
			http.Error(w, "method not allowed", 405)
			return
		}
		rows, e := a.s.orders()
		if e != nil {
			fail(w, 500, e)
			return
		}
		filter := r.URL.Query()
		out := make([]OrderItem, 0, len(rows))
		for _, x := range rows {
			if p := filter.Get("platform"); p != "" && x.Platform != p {
				continue
			}
			if filter.Get("wearable") == "true" && !x.Wearable {
				continue
			}
			if c := filter.Get("category"); c != "" && x.Category != c {
				continue
			}
			if q := strings.ToLower(strings.TrimSpace(filter.Get("q"))); q != "" && !strings.Contains(strings.ToLower(x.Name+" "+x.Spec+" "+x.Shop+" "+x.OrderID), q) {
				continue
			}
			out = append(out, x)
		}
		jsonOut(w, 200, out)
	case "/api/orders/organize":
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in struct {
			Limit int `json:"limit"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&in); e != nil {
			fail(w, 400, e)
			return
		}
		if in.Limit == 0 {
			in.Limit = 100
		}
		result, e := a.s.organizeWearables(in.Limit)
		if e != nil {
			fail(w, 400, e)
			return
		}
		jsonOut(w, 200, result)
	case "/api/orders/cache-images":
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in struct {
			After string `json:"after"`
			Limit int    `json:"limit"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&in); e != nil {
			fail(w, 400, e)
			return
		}
		if len(in.After) > 300 {
			fail(w, 400, errors.New("invalid cursor"))
			return
		}
		if in.Limit == 0 {
			in.Limit = 5
		}
		result, e := a.s.cacheWardrobeImages(r.Context(), in.After, in.Limit, wardrobeImageHTTPClient)
		if e != nil {
			fail(w, 400, e)
			return
		}
		jsonOut(w, 200, result)
	case "/api/orders/import":
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in OrderImport
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&in); e != nil {
			fail(w, 400, e)
			return
		}
		var extra any
		if d.Decode(&extra) != nil && !errors.Is(d.Decode(&extra), nil) { /* handled below */
		}
		// We only accept the documented normalized schema, never raw platform payloads.
		result, e := a.s.importOrders(in)
		if e != nil {
			fail(w, 400, e)
			return
		}
		jsonOut(w, 200, result)
	case "/api/orders/decision":
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in struct {
			SourceKey string  `json:"source_key"`
			Category  *string `json:"category"`
			Owned     *string `json:"owned"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&in); e != nil {
			fail(w, 400, e)
			return
		}
		row, e := a.s.updateOrderDecision(&in.SourceKey, in.Category, in.Owned)
		if e != nil {
			if e == sql.ErrNoRows {
				fail(w, 404, e)
			} else {
				fail(w, 400, e)
			}
			return
		}
		jsonOut(w, 200, row)
	case "/api/orders/promote":
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		var in struct {
			SourceKey string `json:"source_key"`
		}
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		if e := d.Decode(&in); e != nil {
			fail(w, 400, e)
			return
		}
		item, e := a.s.promoteOrderItem(in.SourceKey)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				fail(w, 404, e)
			} else {
				fail(w, 400, e)
			}
			return
		}
		jsonOut(w, 200, item)
	default:
		http.NotFound(w, r)
	}
}

// Sort for deterministic fixture assertions.
