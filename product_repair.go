package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

type WardrobeRepairAction struct {
	ID     string `json:"id"`
	Target string `json:"target,omitempty"`
	Reason string `json:"reason"`
}
type WardrobeRepairResult struct {
	Actions                 []WardrobeRepairAction `json:"actions"`
	Review                  []string               `json:"review"`
	Token                   string                 `json:"token"`
	Applied                 int                    `json:"applied"`
	ProtectedOutfitProducts int                    `json:"protected_outfit_products"`
}

func referencesProduct(v any, id string) bool {
	switch x := v.(type) {
	case string:
		return x == id
	case []any:
		for _, y := range x {
			if referencesProduct(y, id) {
				return true
			}
		}
	case map[string]any:
		for k, y := range x {
			if k == id || referencesProduct(y, id) {
				return true
			}
		}
	}
	return false
}

// Only untouched automatic candidates may be removed from the active catalog.
// Existing images remain on disk, and the complete record is archived.
func pristineImported(p record) bool {
	if !strings.HasPrefix(productText(p["id"]), "IMP-") || productText(p["sourceKey"]) == "" || productText(p["owned"]) != "待确认" || productText(p["fit"]) != "待确认" || productText(p["comment"]) != "" || productText(p["ratingSource"]) != "unrated" || p["rating"] != float64(0) {
		return false
	}
	if productText(p["color"]) != "" || productText(p["size"]) != "" {
		return false
	}
	f := recordFields(p)
	for k := range f {
		switch k {
		case "购买平台", "订单编号", "来源唯一键", "成交核验", "订单状态", "退款状态", "退货状态", "店铺", "原订单商品键", "商品金额", "实付金额", "商品数量", "图片来源", "图片分辨率":
		default:
			return false
		}
	}
	return true
}

func (s *store) repairWardrobe(applyToken string) (WardrobeRepairResult, error) {
	s.productMu.Lock()
	defer s.productMu.Unlock()
	out := WardrobeRepairResult{Actions: []WardrobeRepairAction{}, Review: []string{}}
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT kind,id,body,version FROM records ORDER BY id")
	if err != nil {
		return out, err
	}
	products := map[string]record{}
	raw := map[string]string{}
	versions := map[string]int{}
	others := []any{}
	looks := []any{}
	for rows.Next() {
		var kind, id, body string
		var version int
		if err = rows.Scan(&kind, &id, &body, &version); err != nil {
			rows.Close()
			return out, err
		}
		var p record
		if err = json.Unmarshal([]byte(body), &p); err != nil {
			rows.Close()
			return out, err
		}
		if kind == "products" {
			products[id] = p
			raw[id] = body
			versions[id] = version
		} else {
			others = append(others, map[string]any(p))
			if kind == "looks" {
				looks = append(looks, map[string]any(p))
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = tx.Query("SELECT source_key,platform,order_id,name,spec,url,ordered_at,category,wearable,category_manual,owned,product_id FROM order_items WHERE product_id<>'' ORDER BY source_key")
	if err != nil {
		return out, err
	}
	linked := map[string][]OrderItem{}
	for rows.Next() {
		var i OrderItem
		var w, m int
		if err = rows.Scan(&i.SourceKey, &i.Platform, &i.OrderID, &i.Name, &i.Spec, &i.URL, &i.OrderedAt, &i.Category, &w, &m, &i.Owned, &i.ProductID); err != nil {
			rows.Close()
			return out, err
		}
		i.Wearable = w == 1
		i.CategoryManual = m == 1
		if !i.CategoryManual {
			i.Category, i.Wearable = classifyOrder(i.Name, i.Spec)
		}
		linked[i.ProductID] = append(linked[i.ProductID], i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	ids := make([]string, 0, len(products))
	for id := range products {
		ids = append(ids, id)
	}
	// Retain every product referenced by an existing outfit; prefer those
	// records to unreferenced imported candidates when choosing a canonical ID.
	outfitProtected := map[string]bool{}
	for _, id := range ids {
		for _, look := range looks {
			if referencesProduct(look, id) {
				outfitProtected[id] = true
				break
			}
		}
	}
	out.ProtectedOutfitProducts = len(outfitProtected)
	sort.Slice(ids, func(i, j int) bool {
		if outfitProtected[ids[i]] != outfitProtected[ids[j]] {
			return outfitProtected[ids[i]]
		}
		a, b := strings.HasPrefix(ids[i], "IMP-"), strings.HasPrefix(ids[j], "IMP-")
		if a != b {
			return !a
		}
		return ids[i] < ids[j]
	})
	kept := []record{}
	for _, id := range ids {
		p := products[id]
		orders := linked[id]
		safe := pristineImported(p) && len(orders) > 0
		allExcluded := len(orders) > 0
		for _, i := range orders {
			if i.CategoryManual || i.Owned != "unknown" {
				safe = false
			}
			if i.Wearable {
				allExcluded = false
			}
			// Do not discard edited product attributes even if the record is unrated.
			if productText(p["name"]) != i.Name || productText(p["spec"]) != i.Spec || productText(p["url"]) != i.URL {
				safe = false
			}
		}
		// Exact legacy duplicates can also be consolidated. Their original
		// measurements, ratings and other evidence are retained in the archive.
		referenced := false
		for otherID, other := range products {
			if otherID != id && referencesProduct(map[string]any(other), id) {
				referenced = true
				safe = false
				break
			}
		}
		for _, v := range others {
			if referencesProduct(v, id) {
				referenced = true
				safe = false
				break
			}
		}
		// A legacy record without an order link can still be an exact
		// duplicate. Require a verified variant ID and identical explicit spec.
		if !referenced && !allExcluded && productText(p["sourceKey"]) == "" {
			sku := privateProductSKU(productText(p["url"]))
			spec := normalizedProductSpec(recordSpec(p))
			if sku != "" && spec != "" {
				for _, candidate := range kept {
					if sku != privateProductSKU(productText(candidate["url"])) || spec != normalizedProductSpec(recordSpec(candidate)) || productText(p["name"]) != productText(candidate["name"]) {
						continue
					}
					target := productText(candidate["id"])
					compatible := true
					for _, i := range orders {
						same, maybe := compareWardrobeProduct(i, candidate)
						if !same || maybe {
							compatible = false
							break
						}
					}
					if compatible {
						out.Actions = append(out.Actions, WardrobeRepairAction{ID: id, Target: target, Reason: "duplicate"})
						break
					}
				}
				if len(out.Actions) > 0 && out.Actions[len(out.Actions)-1].ID == id {
					continue
				}
			}
		}
		if allExcluded {
			if safe {
				out.Actions = append(out.Actions, WardrobeRepairAction{ID: id, Reason: "non-wearable"})
				continue
			}
			out.Review = append(out.Review, id)
			continue
		}
		var target string
		uncertain := false
		if len(orders) > 0 {
			for _, i := range orders {
				if !i.Wearable {
					uncertain = true
					break
				}
				match, maybe := matchWardrobeProduct(i, kept)
				if maybe {
					uncertain = true
					break
				}
				if match == nil {
					if target != "" {
						uncertain = true
					}
					target = ""
					break
				}
				mid := productText(match["id"])
				if target != "" && target != mid {
					uncertain = true
					break
				}
				target = mid
			}
		}
		if target != "" && !uncertain && safe {
			out.Actions = append(out.Actions, WardrobeRepairAction{ID: id, Target: target, Reason: "duplicate"})
			continue
		}
		if uncertain || target != "" {
			out.Review = append(out.Review, id)
		}
		kept = append(kept, p)
	}
	// Bind the confirmation to the exact planned records and their versions.
	digest := sha256.New()
	for _, a := range out.Actions {
		b, _ := json.Marshal([]any{a, versions[a.ID], versions[a.Target], linked[a.ID]})
		digest.Write(b)
	}
	out.Token = hex.EncodeToString(digest.Sum(nil))
	if applyToken == "" {
		return out, nil
	}
	if applyToken != out.Token {
		return out, errors.New("repair preview changed; preview again")
	}
	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS wardrobe_product_archive (id TEXT PRIMARY KEY, body TEXT NOT NULL, version INTEGER NOT NULL, target_id TEXT NOT NULL, reason TEXT NOT NULL, order_links TEXT NOT NULL, archived_at TEXT NOT NULL)`); err != nil {
		return out, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, a := range out.Actions {
		b, _ := json.Marshal(linked[a.ID])
		if _, err = tx.Exec("INSERT INTO wardrobe_product_archive VALUES(?,?,?,?,?,?,?)", a.ID, raw[a.ID], versions[a.ID], a.Target, a.Reason, string(b), now); err != nil {
			return out, err
		}
		if a.Target != "" {
			if _, err = tx.Exec("UPDATE product_import_keys SET product_id=? WHERE product_id=?", a.Target, a.ID); err != nil {
				return out, err
			}
			// Retain an available image without overwriting the canonical record's image.
			target := products[a.Target]
			if productText(target["image"]) == "" && productText(products[a.ID]["image"]) != "" {
				target["image"] = products[a.ID]["image"]
				data, _ := json.Marshal(target)
				if _, err = tx.Exec("UPDATE records SET body=?,version=version+1 WHERE kind='products' AND id=?", string(data), a.Target); err != nil {
					return out, err
				}
			}
		} else {
			if _, err = tx.Exec("DELETE FROM product_import_keys WHERE product_id=?", a.ID); err != nil {
				return out, err
			}
		}
		if _, err = tx.Exec("UPDATE order_items SET product_id=?,updated_at=? WHERE product_id=?", a.Target, now, a.ID); err != nil {
			return out, err
		}
		if a.Target == "" {
			for _, i := range linked[a.ID] {
				if _, err = tx.Exec("UPDATE order_items SET category='',wearable=0 WHERE source_key=? AND category_manual=0", i.SourceKey); err != nil {
					return out, err
				}
			}
		}
		res, e := tx.Exec("DELETE FROM records WHERE kind='products' AND id=? AND version=?", a.ID, versions[a.ID])
		if e != nil {
			return out, e
		}
		n, e := res.RowsAffected()
		if e != nil || n != 1 {
			return out, errConflict
		}
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	out.Applied = len(out.Actions)
	return out, nil
}
