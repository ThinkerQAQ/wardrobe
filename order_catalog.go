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
		done++
		_, err := s.promoteOrderItem(v.SourceKey)
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
	sort.Slice(rows, func(i, j int) bool { return rows[i].SourceKey < rows[j].SourceKey })
	candidates := make([]OrderItem, 0)
	for _, v := range rows {
		if v.SourceKey <= after || v.ProductID == "" || v.ImageURL == "" || !v.Wearable || v.Owned == "returned" || v.Owned == "not_owned" {
			continue
		}
		product, e := s.get("products", v.ProductID)
		if e != nil {
			continue
		}
		image, _ := product["image"].(string)
		if image != "" {
			continue
		}
		candidates = append(candidates, v)
	}
	if len(candidates) == 0 {
		return out, nil
	}
	if len(candidates) > limit {
		out.HasMore = true
		candidates = candidates[:limit]
	}
	for _, v := range candidates {
		out.Attempted++
		out.Next = v.SourceKey
		urls := imageCandidates(v.ImageURL)
		if len(urls) == 0 {
			out.MissingSource++
			continue
		}
		product, e := s.get("products", v.ProductID)
		if e != nil {
			out.Failed++
			continue
		}
		version, ok := product["version"].(int)
		if !ok {
			out.Failed++
			continue
		}
		success := false
		failureCategory := "content"
		for _, link := range urls {
			data, fetchErr := fetchOrderImage(ctx, link, client)
			if fetchErr != nil {
				var statusErr orderImageHTTPError
				switch {
				case errors.As(fetchErr, &statusErr):
					failureCategory = "http"
				case errors.Is(fetchErr, context.DeadlineExceeded), errors.Is(fetchErr, context.Canceled):
					failureCategory = "network"
				default:
					var netErr interface{ Timeout() bool }
					if errors.As(fetchErr, &netErr) && netErr.Timeout() {
						failureCategory = "network"
					}
				}
				continue
			}
			_, e = s.saveProductImage(SaveImageInput{ID: v.ProductID, Version: version, ImageBase64: base64.StdEncoding.EncodeToString(data), Source: v.Platform + "订单商品图片（本机 CDN 归档）"})
			if e == nil {
				success = true
				break
			}
			failureCategory = "save"
		}
		if success {
			out.Cached++
		} else {
			out.Failed++
			switch failureCategory {
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
