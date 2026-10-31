package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var importKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_:\-./]{5,180}$`)

type CreateProductInput struct {
	SourceKey    string            `json:"source_key" jsonschema:"唯一成交行凭据，如 source:orderID:lineID；相同来源行不得重复导入"`
	Platform     string            `json:"platform" jsonschema:"实际来源标识；不能凭猜测虚构订单"`
	OrderID      string            `json:"order_id" jsonschema:"有订单时必须提供真实编号；手动记录可空"`
	Verification string            `json:"verification" jsonschema:"核实状态: verified 或 manual。退款和实际持有状态要单独确认"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Color        string            `json:"color,omitempty"`
	Size         string            `json:"size,omitempty"`
	Spec         string            `json:"spec,omitempty"`
	Date         string            `json:"date,omitempty" jsonschema:"YYYY-MM-DD，无法确认可留空"`
	URL          string            `json:"url,omitempty"`
	Fields       map[string]string `json:"fields,omitempty" jsonschema:"原订单/尺码/退款/商品价证据，仅保存核实内容"`
}
type ProductPatchInput struct {
	ID       string            `json:"id"`
	Version  int               `json:"version"`
	Name     *string           `json:"name,omitempty"`
	Category *string           `json:"category,omitempty"`
	Color    *string           `json:"color,omitempty"`
	Size     *string           `json:"size,omitempty"`
	Spec     *string           `json:"spec,omitempty"`
	Date     *string           `json:"date,omitempty"`
	URL      *string           `json:"url,omitempty"`
	Fields   map[string]string `json:"fields,omitempty" jsonschema:"只追加或更新指定证据字段；不清除其他字段、不改变原评分"`
}
type SaveImageInput struct {
	ID          string `json:"id"`
	Version     int    `json:"version" jsonschema:"最近一次读取的商品记录版本"`
	ImageBase64 string `json:"image_base64" jsonschema:"JPEG/PNG 图片字节的 Base64；不能提供任意本地文件路径"`
	Source      string `json:"source" jsonschema:"图片来源与核验说明，如 商品页主图 / 订单快照"`
}

type ImageSaved struct {
	ID      string `json:"id"`
	Image   string `json:"image"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Version int    `json:"version"`
}

func (s *store) importProduct(in CreateProductInput) (record, error) {
	if !importKeyPattern.MatchString(in.SourceKey) {
		return nil, errors.New("invalid unique source_key")
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 500 || strings.TrimSpace(in.Category) == "" || len(in.Category) > 60 {
		return nil, errors.New("name and category required")
	}
	if !validPlatform(in.Platform) {
		return nil, errors.New("invalid platform")
	}
	if in.Platform == "手动" {
		if in.Verification != "manual" {
			return nil, errors.New("manual record requires manual verification")
		}
	} else if in.OrderID == "" || in.Verification != "verified" {
		return nil, errors.New("real order_id and verified status required")
	}
	for _, x := range []string{in.Color, in.Size, in.Spec, in.Date, in.URL} {
		if len(x) > 2000 {
			return nil, errors.New("field too long")
		}
	}
	if in.Date != "" {
		if _, e := time.Parse("2006-01-02", in.Date); e != nil {
			return nil, errors.New("invalid date")
		}
	}
	if in.URL != "" && !(strings.HasPrefix(in.URL, "https://") || strings.HasPrefix(in.URL, "http://")) {
		return nil, errors.New("invalid link")
	}
	if len(in.Fields) > 100 {
		return nil, errors.New("too many evidence fields")
	}
	for k, v := range in.Fields {
		if len(k) > 100 || len(v) > 5000 {
			return nil, errors.New("evidence field too long")
		}
	}
	b := make([]byte, 8)
	if _, e := rand.Read(b); e != nil {
		return nil, e
	}
	id := "IMP-" + hex.EncodeToString(b)
	fields := map[string]string{"购买平台": in.Platform, "订单编号": in.OrderID, "来源唯一键": in.SourceKey, "成交核验": in.Verification}
	for k, v := range in.Fields {
		if _, reserved := fields[k]; reserved {
			return nil, errors.New("reserved evidence field " + k)
		}
		fields[k] = v
	}
	p := record{"id": id, "name": in.Name, "category": in.Category, "color": in.Color, "size": in.Size, "spec": in.Spec, "date": in.Date, "url": in.URL, "fields": fields, "sourceKey": in.SourceKey, "image": "", "owned": "待确认", "fit": "待确认", "rating": 0, "ratingSource": "unrated", "comment": "", "updatedAt": time.Now().UTC().Format(time.RFC3339)}
	raw, _ := json.Marshal(p)
	tx, e := s.db.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("INSERT INTO product_import_keys(source_key,product_id) VALUES(?,?)", in.SourceKey, id); e != nil {
		return nil, fmt.Errorf("already imported or invalid source key: %w", errConflict)
	}
	if _, e = tx.Exec("INSERT INTO records(kind,id,body,version) VALUES('products',?,?,1)", id, string(raw)); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return s.get("products", id)
}
func (s *store) patchProduct(in ProductPatchInput) (record, error) {
	if !validID.MatchString(in.ID) || in.Version < 1 {
		return nil, errors.New("valid id/version required")
	}
	p, e := s.get("products", in.ID)
	if e != nil {
		return nil, e
	}
	updates := map[string]*string{"name": in.Name, "category": in.Category, "color": in.Color, "size": in.Size, "spec": in.Spec, "date": in.Date, "url": in.URL}
	for k, v := range updates {
		if v == nil {
			continue
		}
		if len(*v) > 2000 {
			return nil, errors.New("field too long")
		}
		if (k == "name" || k == "category") && strings.TrimSpace(*v) == "" {
			return nil, errors.New("required product field empty")
		}
		p[k] = *v
	}
	if in.Fields != nil {
		existing, ok := p["fields"].(map[string]any)
		if !ok {
			existing = map[string]any{}
		}
		for k, v := range in.Fields {
			if len(k) > 100 || len(v) > 5000 {
				return nil, errors.New("evidence field too long")
			}
			if k == "来源唯一键" || k == "购买平台" || k == "订单编号" {
				return nil, errors.New("order identity is immutable")
			}
			existing[k] = v
		}
		p["fields"] = existing
	}
	return s.save("products", in.ID, p, in.Version)
}
func (s *store) saveProductImage(in SaveImageInput) (ImageSaved, error) {
	out := ImageSaved{}
	if !validID.MatchString(in.ID) || in.Version < 1 {
		return out, errors.New("invalid id/version")
	}
	if in.Source == "" || len(in.Source) > 500 {
		return out, errors.New("image source required")
	}
	if len(in.ImageBase64) > 12<<20 {
		return out, errors.New("encoded image too large")
	}
	data, e := base64.StdEncoding.DecodeString(in.ImageBase64)
	if e != nil {
		return out, e
	}
	if len(data) > 8<<20 {
		return out, errors.New("image too large")
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil {
		return out, e
	}
	if (format != "jpeg" && format != "png" && format != "webp") || cfg.Width < 60 || cfg.Height < 60 || cfg.Width > 6000 || cfg.Height > 6000 || int64(cfg.Width)*int64(cfg.Height) > 24_000_000 {
		return out, errors.New("invalid image format/dimensions")
	}
	im, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return out, e
	}
	current, e := s.get("products", in.ID)
	if e != nil {
		return out, e
	}
	if current["version"] != in.Version {
		return out, errConflict
	}
	folder := filepath.Join(s.dir, "images")
	if e = os.MkdirAll(folder, 0700); e != nil {
		return out, e
	}
	tmp, e := os.CreateTemp(folder, ".import-*.jpg")
	if e != nil {
		return out, e
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if e = jpeg.Encode(tmp, im, &jpeg.Options{Quality: 90}); e != nil {
		tmp.Close()
		return out, e
	}
	if e = tmp.Close(); e != nil {
		return out, e
	}
	dest := filepath.Join(folder, in.ID+".jpg")
	if prev, e := os.ReadFile(dest); e == nil {
		backupFolder := filepath.Join(s.dir, "backups", "image-replacements")
		if e = os.MkdirAll(backupFolder, 0700); e != nil {
			return out, e
		}
		backup := filepath.Join(backupFolder, in.ID+"-"+time.Now().UTC().Format("20060102T150405.000000000")+".jpg")
		if e = os.WriteFile(backup, prev, 0600); e != nil {
			return out, e
		}
	}
	if e = os.Rename(tmpPath, dest); e != nil {
		return out, e
	}
	current["image"] = "/media/images/" + in.ID + ".jpg"
	fields, ok := current["fields"].(map[string]any)
	if !ok {
		fields = map[string]any{}
	}
	fields["图片来源"] = in.Source
	fields["图片分辨率"] = fmt.Sprintf("%d×%d", cfg.Width, cfg.Height)
	current["fields"] = fields
	saved, e := s.save("products", in.ID, current, in.Version)
	if e != nil {
		return out, e
	}
	out = ImageSaved{ID: in.ID, Image: saved["image"].(string), Width: cfg.Width, Height: cfg.Height, Version: saved["version"].(int)}
	return out, nil
}

type AuditResult struct {
	ProductCount        int      `json:"product_count"`
	HairstyleCount      int      `json:"hairstyle_count"`
	MissingImages       []string `json:"missing_images"`
	LowResolution       []string `json:"low_resolution"`
	DuplicateSourceKeys []string `json:"duplicate_source_keys"`
}

func (s *store) auditWardrobe() (AuditResult, error) {
	out := AuditResult{MissingImages: []string{}, LowResolution: []string{}, DuplicateSourceKeys: []string{}}
	seen := map[string]bool{}
	for _, kind := range []string{"products", "hairstyles"} {
		rows, e := s.list(kind)
		if e != nil {
			return out, e
		}
		if kind == "products" {
			out.ProductCount = len(rows)
		} else {
			out.HairstyleCount = len(rows)
		}
		dir := "images"
		if kind == "hairstyles" {
			dir = "hairstyles"
		}
		for _, r := range rows {
			id, _ := r["id"].(string)
			if key, _ := r["sourceKey"].(string); key != "" {
				if seen[key] {
					out.DuplicateSourceKeys = append(out.DuplicateSourceKeys, id)
				}
				seen[key] = true
			}
			if r["image"] == "" {
				out.MissingImages = append(out.MissingImages, id)
				continue
			}
			f, e := os.Open(filepath.Join(s.dir, dir, id+".jpg"))
			if e != nil {
				out.MissingImages = append(out.MissingImages, id)
				continue
			}
			cfg, _, e := image.DecodeConfig(f)
			f.Close()
			if e != nil {
				out.MissingImages = append(out.MissingImages, id)
				continue
			}
			if cfg.Width < 500 || cfg.Height < 500 {
				out.LowResolution = append(out.LowResolution, id)
			}
		}
	}
	return out, nil
}

var _ sql.Result
