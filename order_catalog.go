package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	_ "golang.org/x/image/webp"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// These operations only transform already normalized local order rows. They
// never mark an uncertain purchase as actually owned.
type OrderOrganizeResult struct {
	Eligible  int `json:"eligible"`
	Linked    int `json:"linked"`
	Created   int `json:"created"`
	Remaining int `json:"remaining"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
	Review    int `json:"review"`
}

// New, conservative clothing keywords are applied only to automatic category
// decisions. The user's manual category overrides remain immutable.
func (s *store) refreshAutomaticOrderCategories() error {
	rows, err := s.db.Query(`SELECT source_key,name,spec,category,wearable FROM order_items WHERE category_manual=0`)
	if err != nil {
		return err
	}
	type item struct {
		key, category string
		wearable      bool
	}
	updates := []item{}
	for rows.Next() {
		var key, name, spec, current string
		var active int
		if err = rows.Scan(&key, &name, &spec, &current, &active); err != nil {
			rows.Close()
			return err
		}
		cat, wearable := classifyOrder(name, spec)
		if cat != current || wearable != (active == 1) {
			updates = append(updates, item{key, cat, wearable})
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(updates) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, v := range updates {
		flag := 0
		if v.wearable {
			flag = 1
		}
		if _, err = tx.Exec(`UPDATE order_items SET category=?,wearable=? WHERE source_key=? AND category_manual=0`, v.category, flag, v.key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *store) organizeWearables(limit int) (OrderOrganizeResult, error) {
	out := OrderOrganizeResult{}
	if limit < 1 || limit > 150 {
		return out, errors.New("organize limit must be 1..150")
	}
	for round := 0; round < 30; round++ {
		reconciled, err := privateReconcileOrder(s, 150)
		if err != nil {
			return out, err
		}
		if reconciled.Eligible <= reconciled.Merged {
			break
		}
		if reconciled.Merged == 0 {
			break
		}
	}
	if err := s.refreshAutomaticOrderCategories(); err != nil {
		return out, err
	}
	rows, err := s.orders()
	if err != nil {
		return out, err
	}
	for _, v := range rows {
		if !v.Wearable || v.Owned == "returned" || v.Owned == "not_owned" {
			continue
		}
		out.Eligible++
		if v.ProductID != "" {
			out.Linked++
			continue
		}
		out.Remaining++
	}
	if out.Remaining == 0 {
		return out, nil
	}
	prior, err := s.list("products")
	if err != nil {
		return out, err
	}
	priorCount := len(prior)
	done := 0
	for _, v := range rows {
		if done >= limit {
			break
		}
		if !v.Wearable || v.ProductID != "" || v.Owned == "returned" || v.Owned == "not_owned" {
			continue
		}
		_, err := s.promoteOrderItem(v.SourceKey)
		if errors.Is(err, errProductReview) {
			out.Review++
			continue
		}
		done++
		if err != nil {
			out.Errors++
			continue
		}
		out.Remaining--
		out.Linked++
	}
	now, err := s.list("products")
	if err != nil {
		return out, err
	}
	out.Created = len(now) - priorCount
	return out, nil
}

type OrderImageResult struct {
	Attempted       int    `json:"attempted"`
	Cached          int    `json:"cached"`
	Failed          int    `json:"failed"`
	MissingSource   int    `json:"missing_source"`
	HTTPFailures    int    `json:"http_failures"`
	NetworkFailures int    `json:"network_failures"`
	ContentFailures int    `json:"content_failures"`
	SaveFailures    int    `json:"save_failures"`
	Next            string `json:"next"`
	HasMore         bool   `json:"has_more"`
}

// No remote order-image hosts are enabled in public builds.
var allowedOrderImageHosts = []string{}
var orderImageVariants = func(cleaned string) []string { return []string{cleaned} }

func validOrderImageURL(raw string) (string, error) {
	if len(raw) > 2048 {
		return "", errors.New("image URL too long")
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return "", errors.New("invalid HTTPS image URL")
	}
	host := strings.ToLower(u.Hostname())
	allowed := false
	for _, domain := range allowedOrderImageHosts {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			allowed = true
			break
		}
	}
	if !allowed || strings.Contains(u.EscapedPath(), "..") || u.Path == "" {
		return "", errors.New("image host/path not allowlisted")
	}
	u.RawQuery = ""
	u.ForceQuery = false
	return u.String(), nil
}

var wardrobeImageHTTPClient = &http.Client{
	Timeout:       8 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
}

func imageCandidates(raw string) []string {
	cleaned, err := validOrderImageURL(raw)
	if err != nil {
		return nil
	}
	result := []string{}
	seen := map[string]bool{}
	for _, candidate := range append(orderImageVariants(cleaned), cleaned) {
		if len(result) >= 3 {
			break
		}
		safe, e := validOrderImageURL(candidate)
		if e == nil && !seen[safe] {
			result = append(result, safe)
			seen[safe] = true
		}
	}
	return result
}

// Deliberately omit the URL from HTTP errors; order source links are private.
type orderImageHTTPError struct{ Status int }

func (e orderImageHTTPError) Error() string { return fmt.Sprintf("image response status %d", e.Status) }

func fetchOrderImage(ctx context.Context, raw string, client *http.Client) ([]byte, error) {
	cleaned, err := validOrderImageURL(raw)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", cleaned, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/webp")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, orderImageHTTPError{Status: resp.StatusCode}
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(ct, "image/jpeg") && !strings.HasPrefix(ct, "image/png") && !strings.HasPrefix(ct, "image/webp") && !strings.HasPrefix(ct, "application/octet-stream") {
		return nil, errors.New("not an image response")
	}
	if resp.ContentLength > 5<<20 {
		return nil, errors.New("image exceeds limit")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (5<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 5<<20 || len(data) < 500 {
		return nil, errors.New("invalid image size")
	}
	return data, nil
}
func (s *store) cacheWardrobeImages(ctx context.Context, after string, limit int, client *http.Client) (OrderImageResult, error) {
	out := OrderImageResult{}
	if limit < 1 || limit > 10 {
		return out, errors.New("image batch limit must be 1..10")
	}
	rows, err := s.orders()
	if err != nil {
		return out, err
	}
	products, err := s.list("products")
	if err != nil {
		return out, err
	}
	sort.Slice(products, func(i, j int) bool { return productText(products[i]["id"]) < productText(products[j]["id"]) })
	linked := map[string][]OrderItem{}
	for _, r := range rows {
		if r.ImageURL != "" && r.ProductID != "" {
			linked[r.ProductID] = append(linked[r.ProductID], r)
		}
	}
	for _, p := range products {
		id := productText(p["id"])
		if id <= after || productText(p["image"]) != "" {
			continue
		}
		candidates := append([]OrderItem{}, linked[id]...)
		// Older records often have no order link. Only reuse a verified SKU or
		// matching listing+spec, never a similar title or a generic image.
		if len(candidates) == 0 {
			for _, r := range rows {
				if r.ImageURL != "" {
					same, maybe := compareWardrobeProduct(r, p)
					if same && !maybe {
						candidates = append(candidates, r)
					}
				}
			}
		}
		legacyURL := productText(recordFields(p)["历史订单主图链接"])
		if len(candidates) == 0 && legacyURL == "" {
			continue
		}
		if out.Attempted >= limit {
			out.HasMore = true
			break
		}
		out.Attempted++
		out.Next = id
		success := false
		failure := ""
		seen := map[string]bool{}
		save := func(data []byte, source string) bool {
			_, e := s.saveProductImage(SaveImageInput{ID: id, Version: p["version"].(int), ImageBase64: base64.StdEncoding.EncodeToString(data), Source: source})
			if e != nil {
				failure = "save"
				return false
			}
			return true
		}
		// Prefer the already validated local order thumbnail to another CDN request.
		for _, r := range candidates {
			path := s.orderThumbnailPath(r.SourceKey, r.ImageURL)
			if b, e := os.ReadFile(path); e == nil && len(b) <= 8<<20 && save(b, r.Platform+"订单缩略图（本地复用）") {
				success = true
				break
			}
			if r.ProductID != "" && r.ProductID != id && validID.MatchString(r.ProductID) {
				if b, e := os.ReadFile(filepath.Join(s.dir, "images", r.ProductID+".jpg")); e == nil && len(b) <= 8<<20 && save(b, r.Platform+"同款订单图（本地复用）") {
					success = true
					break
				}
			}
		}
		urls := []string{}
		for _, r := range candidates {
			urls = append(urls, r.ImageURL)
		}
		if f := recordFields(p); f != nil {
			urls = append(urls, productText(f["历史订单主图链接"]))
		}
		for _, raw := range urls {
			if success {
				break
			}
			for _, link := range imageCandidates(raw) {
				if seen[link] {
					continue
				}
				seen[link] = true
				b, e := fetchOrderImage(ctx, link, client)
				if e != nil {
					var se orderImageHTTPError
					if errors.As(e, &se) {
						failure = "http"
					} else {
						var netErr interface{ Timeout() bool }
						failure = "content"
						if errors.Is(e, context.DeadlineExceeded) || errors.Is(e, context.Canceled) || errors.As(e, &netErr) && netErr.Timeout() {
							failure = "network"
						}
					}
					continue
				}
				if save(b, "订单商品图（本机 CDN 归档）") {
					success = true
					break
				}
			}
		}
		if success {
			out.Cached++
		} else if failure == "" {
			out.MissingSource++
		} else {
			out.Failed++
			switch failure {
			case "http":
				out.HTTPFailures++
			case "network":
				out.NetworkFailures++
			case "save":
				out.SaveFailures++
			default:
				out.ContentFailures++
			}
		}
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
	}
	return out, nil
}
