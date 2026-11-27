package main

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

var errProductReview = errors.New("possible duplicate product: verify SKU before promoting")

// Optional source adapter: only return an identity when the URL is known to
// identify a single purchasable variant rather than a listing/SPU.
var privateProductSKU = func(string) string { return "" }
var privateProductListing = func(string) string { return "" }

func productText(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
func normalizedProductSpec(v string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("·;；,，/|、", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, v)
}
func normalizedProductURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	u.Scheme = "https"
	u.Fragment = ""
	q := u.Query()
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			q.Del(k)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}
func recordSpec(p record) string {
	if v := productText(p["spec"]); v != "" {
		return v
	}
	return strings.TrimSpace(productText(p["color"]) + " " + productText(p["size"]))
}
func recordFields(p record) map[string]any {
	f, _ := p["fields"].(map[string]any)
	return f
}

// Only positive evidence matches. Same listing with an unknown specification
// is deliberately held for review instead of silently creating another item.
func compareWardrobeProduct(i OrderItem, p record) (same, uncertain bool) {
	f := recordFields(p)
	sameTitle := strings.TrimSpace(i.Name) == productText(p["name"])
	oldURL, newURL := normalizedProductURL(productText(p["url"])), normalizedProductURL(i.URL)
	sameURL := oldURL != "" && oldURL == newURL
	listing := privateProductListing(i.URL)
	sameURL = sameURL || listing != "" && listing == privateProductListing(productText(p["url"]))
	sku := privateProductSKU(i.URL)
	sameSKU := sku != "" && sku == privateProductSKU(productText(p["url"]))
	samePurchase := i.OrderID != "" && productText(f["订单编号"]) == i.OrderID && productText(f["购买平台"]) == i.Platform && sameTitle
	if !sameURL && !sameSKU && !samePurchase {
		return false, false
	}
	a, b := normalizedProductSpec(i.Spec), normalizedProductSpec(recordSpec(p))
	if a != "" && b != "" && a != b {
		// Different variants of the same listing are legitimate distinct items.
		// A verified SKU or same purchase with conflicting evidence requires review.
		return false, sameSKU || samePurchase
	}
	if sameSKU && (sameTitle || a != "" && a == b) {
		return true, false
	}
	if a != "" && a == b && (sameURL || samePurchase) {
		return true, false
	}
	// A legacy record may lack a purchase ID, while a source-backed record
	// with no spec may represent a different line. Never guess by name alone.
	if samePurchase && productText(p["sourceKey"]) == "" && sameTitle {
		return true, false
	}
	return false, true
}

func matchWardrobeProduct(i OrderItem, prior []record) (record, bool) {
	var match record
	var exactDate record
	dateCount := 0
	matches := 0
	uncertain := false
	for _, p := range prior {
		same, maybe := compareWardrobeProduct(i, p)
		if maybe {
			uncertain = true
		}
		if !same {
			continue
		}
		matches++
		if dateOnly(i.OrderedAt) != "" && dateOnly(i.OrderedAt) == productText(p["date"]) {
			exactDate = p
			dateCount++
		}
		if match == nil {
			match = p
		}
	}
	if uncertain {
		return nil, true
	}
	if matches > 1 {
		if dateCount == 1 {
			return exactDate, false
		}
		return nil, true
	}
	return match, false
}
