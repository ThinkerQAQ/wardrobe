package main

import (
	"encoding/json"
	"errors"
	"time"
)

type ProductDeleteSelection struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type ProductDeleteResult struct {
	Deleted   int      `json:"deleted"`
	Protected []string `json:"protected"`
}

func (s *store) deleteWardrobeProducts(items []ProductDeleteSelection) (ProductDeleteResult, error) {
	out := ProductDeleteResult{Protected: []string{}}
	if len(items) == 0 || len(items) > 2000 {
		return out, errors.New("select 1..2000 products")
	}
	s.productMu.Lock()
	defer s.productMu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	rows, e := tx.Query("SELECT body FROM records WHERE kind<>'products'")
	if e != nil {
		return out, e
	}
	refs := []any{}
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return out, e
		}
		var v any
		if e = json.Unmarshal([]byte(b), &v); e != nil {
			rows.Close()
			return out, e
		}
		refs = append(refs, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if _, e = tx.Exec(`CREATE TABLE IF NOT EXISTS wardrobe_product_archive (id TEXT PRIMARY KEY, body TEXT NOT NULL, version INTEGER NOT NULL, target_id TEXT NOT NULL, reason TEXT NOT NULL, order_links TEXT NOT NULL, archived_at TEXT NOT NULL)`); e != nil {
		return out, e
	}
	seen := map[string]bool{}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, i := range items {
		if !validID.MatchString(i.ID) || i.Version < 1 || seen[i.ID] {
			return out, errors.New("invalid selection")
		}
		seen[i.ID] = true
		var body string
		var v int
		if e = tx.QueryRow("SELECT body,version FROM records WHERE kind='products' AND id=?", i.ID).Scan(&body, &v); e != nil {
			return out, e
		}
		if v != i.Version {
			return out, errConflict
		}
		protected := false
		for _, r := range refs {
			if referencesProduct(r, i.ID) {
				protected = true
				break
			}
		}
		if protected {
			out.Protected = append(out.Protected, i.ID)
			continue
		}
		rows, e := tx.Query("SELECT source_key FROM order_items WHERE product_id=?", i.ID)
		if e != nil {
			return out, e
		}
		keys := []string{}
		for rows.Next() {
			var k string
			if e = rows.Scan(&k); e != nil {
				rows.Close()
				return out, e
			}
			keys = append(keys, k)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return out, e
		}
		b, _ := json.Marshal(keys)
		if _, e = tx.Exec("INSERT INTO wardrobe_product_archive VALUES(?,?,?,?,?,?,?)", i.ID, body, v, "", "manual-delete", string(b), now); e != nil {
			return out, e
		}
		if _, e = tx.Exec("UPDATE order_items SET product_id='',category='',wearable=0,category_manual=1,updated_at=? WHERE product_id=?", now, i.ID); e != nil {
			return out, e
		}
		if _, e = tx.Exec("DELETE FROM product_import_keys WHERE product_id=?", i.ID); e != nil {
			return out, e
		}
		res, e := tx.Exec("DELETE FROM records WHERE kind='products' AND id=? AND version=?", i.ID, v)
		if e != nil {
			return out, e
		}
		n, e := res.RowsAffected()
		if e != nil || n != 1 {
			return out, errConflict
		}
		out.Deleted++
	}
	if e = tx.Commit(); e != nil {
		return out, e
	}
	return out, nil
}
