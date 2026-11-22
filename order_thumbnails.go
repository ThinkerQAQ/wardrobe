package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
)

// Order thumbnails are independent of wardrobe inventory. An order does not
// need to be wearable or promoted into the wardrobe to have a local preview.
// Include the URL in the stable ID so changing an item image invalidates cache.
func orderThumbnailID(sourceKey, rawURL string) string {
	hash := sha256.Sum256([]byte(sourceKey + "\x00" + rawURL))
	return "OI-" + hex.EncodeToString(hash[:12])
}

func (s *store) orderThumbnailPath(sourceKey, rawURL string) string {
	return filepath.Join(s.dir, "images", orderThumbnailID(sourceKey, rawURL)+".jpg")
}

func (s *store) localOrderThumbnail(item OrderItem) string {
	if item.ImageURL == "" {
		return ""
	}
	path := s.orderThumbnailPath(item.SourceKey, item.ImageURL)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		return "/media/images/" + orderThumbnailID(item.SourceKey, item.ImageURL) + ".jpg"
	}
	return ""
}

// Preserve the existing image format and size restrictions, and write a JPEG
// atomically so UI requests cannot observe a partially downloaded thumbnail.
func (s *store) saveOrderThumbnail(sourceKey, rawURL string, data []byte) error {
	if len(data) < 500 || len(data) > 5<<20 {
		return errors.New("invalid thumbnail size")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if (format != "jpeg" && format != "png" && format != "webp") ||
		cfg.Width < 60 || cfg.Height < 60 || cfg.Width > 6000 || cfg.Height > 6000 ||
		int64(cfg.Width)*int64(cfg.Height) > 24_000_000 {
		return errors.New("invalid thumbnail image dimensions or format")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	dir := filepath.Join(s.dir, "images")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".order-thumbnail-*.jpg")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 84}); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.orderThumbnailPath(sourceKey, rawURL))
}

// Cache order thumbnails for all supported retailers. Only verified CDN URLs
// may be fetched; no cookies, shopping sessions or arbitrary paths are read.
func (s *store) cacheOrderThumbnails(ctx context.Context, after string, limit int, client *http.Client) (OrderImageResult, error) {
	out := OrderImageResult{}
	if limit < 1 || limit > 5 {
		return out, errors.New("order thumbnail limit must be 1..5")
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT source_key,image_url,product_id FROM order_items WHERE platform IN ('京东','淘宝','拼多多') AND image_url<>'' AND source_key>? ORDER BY source_key", after)
	if err != nil {
		return out, err
	}
	type candidate struct{ key, url, productID string }
	candidates := make([]candidate, 0, limit+1)
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.key, &c.url, &c.productID); err != nil {
			rows.Close()
			return out, err
		}
		if info, e := os.Stat(s.orderThumbnailPath(c.key, c.url)); e == nil && info.Mode().IsRegular() {
			continue
		}
		// Already archived wardrobe photos are also valid order previews.
		if validID.MatchString(c.productID) {
			if info, e := os.Stat(filepath.Join(s.dir, "images", c.productID+".jpg")); e == nil && info.Mode().IsRegular() {
				continue
			}
		}
		candidates = append(candidates, c)
		if len(candidates) > limit {
			out.HasMore = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return out, err
	}
	rows.Close()
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	for _, c := range candidates {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out.Attempted++
		out.Next = c.key
		urls := imageCandidates(c.url)
		if len(urls) == 0 {
			out.MissingSource++
			continue
		}
		failure := "content"
		saved := false
		for _, u := range urls {
			data, e := fetchOrderImage(ctx, u, client)
			if e != nil {
				var httpErr orderImageHTTPError
				switch {
				case errors.As(e, &httpErr):
					failure = "http"
				case errors.Is(e, context.DeadlineExceeded), errors.Is(e, context.Canceled):
					failure = "network"
				default:
					var netErr interface{ Timeout() bool }
					if errors.As(e, &netErr) && netErr.Timeout() {
						failure = "network"
					}
				}
				continue
			}
			if e = s.saveOrderThumbnail(c.key, c.url, data); e == nil {
				saved = true
				break
			}
			failure = "save"
		}
		if saved {
			out.Cached++
			continue
		}
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
	return out, nil
}
