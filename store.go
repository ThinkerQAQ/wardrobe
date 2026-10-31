package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type record map[string]any
type store struct {
	db  *sql.DB
	dir string
}

var validID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
var errConflict = errors.New("record changed; reload before saving")

func newStore(dir string) (*store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "wardrobe.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &store{db, dir}
	_, e = db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS records(kind TEXT NOT NULL,id TEXT NOT NULL,body TEXT NOT NULL,version INTEGER NOT NULL DEFAULT 1,PRIMARY KEY(kind,id)); CREATE TABLE IF NOT EXISTS product_import_keys(source_key TEXT PRIMARY KEY, product_id TEXT NOT NULL); CREATE TABLE IF NOT EXISTS app_settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);")
	if e != nil {
		db.Close()
		return nil, e
	}
	if e = s.ensureOrders(); e != nil {
		db.Close()
		return nil, e
	}
	var n int
	if e = db.QueryRow("SELECT count(*) FROM records").Scan(&n); e != nil {
		return nil, e
	}
	if n == 0 {
		b, e := os.ReadFile(filepath.Join(dir, "seed.json"))
		if e != nil {
			if os.IsNotExist(e) {
				return s, nil
			}
			db.Close()
			return nil, fmt.Errorf("read seed.json: %w", e)
		}
		var seed map[string][]record
		if e = json.Unmarshal(b, &seed); e != nil {
			return nil, e
		}
		tx, e := db.Begin()
		if e != nil {
			return nil, e
		}
		defer tx.Rollback()
		for _, kind := range []string{"products", "looks", "notes"} {
			for _, r := range seed[kind] {
				id, _ := r["id"].(string)
				if !validID.MatchString(id) {
					return nil, fmt.Errorf("invalid seed id %q", id)
				}
				b, _ := json.Marshal(r)
				if _, e = tx.Exec("INSERT INTO records(kind,id,body) VALUES(?,?,?)", kind, id, string(b)); e != nil {
					return nil, e
				}
			}
		}
		if e = tx.Commit(); e != nil {
			return nil, e
		}
	}
	return s, nil
}
func (s *store) list(kind string) ([]record, error) {
	rows, e := s.db.Query("SELECT body,version FROM records WHERE kind=? ORDER BY id", kind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []record{}
	for rows.Next() {
		var b string
		var v int
		if e = rows.Scan(&b, &v); e != nil {
			return nil, e
		}
		var r record
		if e = json.Unmarshal([]byte(b), &r); e != nil {
			return nil, e
		}
		r["version"] = v
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *store) get(kind, id string) (record, error) {
	var b string
	var v int
	e := s.db.QueryRow("SELECT body,version FROM records WHERE kind=? AND id=?", kind, id).Scan(&b, &v)
	if e != nil {
		return nil, e
	}
	var r record
	e = json.Unmarshal([]byte(b), &r)
	r["version"] = v
	return r, e
}
func (s *store) save(kind, id string, r record, version int) (record, error) {
	if !validID.MatchString(id) {
		return nil, errors.New("invalid id")
	}
	r["id"] = id
	r["updatedAt"] = time.Now().UTC().Format(time.RFC3339)
	delete(r, "version")
	b, e := json.Marshal(r)
	if e != nil {
		return nil, e
	}
	if version == 0 {
		_, e = s.db.Exec("INSERT INTO records(kind,id,body,version) VALUES(?,?,?,1)", kind, id, string(b))
		if e != nil {
			return nil, errConflict
		}
	} else {
		result, e := s.db.Exec("UPDATE records SET body=?,version=version+1 WHERE kind=? AND id=? AND version=?", string(b), kind, id, version)
		if e != nil {
			return nil, e
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return nil, errConflict
		}
	}
	return s.get(kind, id)
}

// deleteLook removes only a saved look, never its products or photo files.
// Optimistic locking prevents accidental deletion of a concurrently edited look.
func (s *store) deleteLook(id string, version int) error {
	if !validID.MatchString(id) || version < 1 {
		return errors.New("valid look id and version required")
	}
	res, err := s.db.Exec("DELETE FROM records WHERE kind='looks' AND id=? AND version=?", id, version)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errConflict
	}
	return nil
}
