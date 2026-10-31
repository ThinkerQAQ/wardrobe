package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	cardW    = 1200
	cardH    = 1800
	cardLeft = 792
)

type lookbookRenderInput struct {
	LookID         string            `json:"lookId,omitempty"`
	PortraitLookID string            `json:"portraitLookId,omitempty"`
	PortraitBase64 string            `json:"portraitImageBase64,omitempty"`
	Slots          map[string]string `json:"slots,omitempty"`
	HairID         string            `json:"hairId,omitempty"`
}
type lookbookProduct struct {
	ID, Name, Size, Color string
	Image                 image.Image
}

func readLookbookImage(raw []byte) (image.Image, error) {
	if len(raw) < 1000 || len(raw) > 8<<20 {
		return nil, errors.New("image size must be 1 KiB–8 MiB")
	}
	cfg, typ, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || (typ != "jpeg" && typ != "png") {
		return nil, errors.New("PNG or JPEG image required")
	}
	if cfg.Width < 256 || cfg.Height < 256 || cfg.Width > 9000 || cfg.Height > 9000 || int64(cfg.Width)*int64(cfg.Height) > 24_000_000 {
		return nil, errors.New("image resolution outside safe limits")
	}
	img, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, errors.New("image decoding failed")
	}
	return img, nil
}
func (a *application) cardArchivedImage(kind, id string) (image.Image, error) {
	if !validID.MatchString(id) {
		return nil, errors.New("invalid image record id")
	}
	field, dir := "image", "images"
	switch kind {
	case "products":
	case "looks":
		field, dir = "photo", "photos"
	case "hairstyles":
		dir = "hairstyles"
	default:
		return nil, errors.New("invalid image kind")
	}
	item, e := a.s.get(kind, id)
	if e != nil {
		return nil, fmt.Errorf("record %s not found", id)
	}
	if item[field] != "/media/"+dir+"/"+id+".jpg" {
		return nil, fmt.Errorf("%s has no canonical archived image", id)
	}
	path := filepath.Join(a.s.dir, dir, id+".jpg")
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s archived image missing", id)
	}
	if info.Size() > 8<<20 {
		return nil, fmt.Errorf("%s image exceeds 8 MiB", id)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	return readLookbookImage(b)
}
func (a *application) cardPortrait(in lookbookRenderInput) (image.Image, error) {
	if (in.PortraitLookID == "") == (in.PortraitBase64 == "") {
		return nil, errors.New("choose exactly one portraitLookId or portraitImageBase64")
	}
	if in.PortraitLookID != "" {
		return a.cardArchivedImage("looks", in.PortraitLookID)
	}
	s := in.PortraitBase64
	for _, prefix := range []string{"data:image/jpeg;base64,", "data:image/png;base64,"} {
		s = strings.TrimPrefix(s, prefix)
	}
	if len(s) > 11<<20 {
		return nil, errors.New("encoded portrait too large")
	}
	b, e := base64.StdEncoding.Strict().DecodeString(s)
	if e != nil {
		return nil, errors.New("invalid Base64 portrait")
	}
	return readLookbookImage(b)
}
func cardSlots(look record, overrides map[string]string) (map[string]string, error) {
	slots := map[string]string{}
	if look != nil {
		if raw, ok := look["slots"].(map[string]any); ok {
			for k, v := range raw {
				if s, yes := v.(string); yes {
					slots[k] = s
				}
			}
		}
	}
	allowed := map[string]bool{"outer": true, "mid": true, "inner": true, "pants": true, "shoes": true}
	for k, id := range overrides {
		if !allowed[k] || (id != "" && !validID.MatchString(id)) {
			return nil, errors.New("invalid lookbook slot " + k)
		}
		slots[k] = id
	}
	return slots, nil
}
func (a *application) renderLookbookCard(in lookbookRenderInput) ([]byte, error) {
	if len(in.PortraitBase64) > 11<<20 {
		return nil, errors.New("portrait too large")
	}
	var look record
	if in.LookID != "" {
		if !validID.MatchString(in.LookID) {
			return nil, errors.New("invalid lookId")
		}
		var e error
		look, e = a.s.get("looks", in.LookID)
		if e != nil {
			return nil, errors.New("lookId not found")
		}
	}
	if look == nil && len(in.Slots) == 0 {
		return nil, errors.New("lookId or slots required")
	}
	slots, e := cardSlots(look, in.Slots)
	if e != nil {
		return nil, e
	}
	middle := slots["mid"]
	if middle == "" {
		middle = slots["inner"]
	}
	ids := []string{slots["outer"], middle, slots["pants"], slots["shoes"]}
	names := []string{"外套", "中层 / 内搭", "裤装", "鞋履"}
	products := make([]lookbookProduct, 0, 4)
	for i, id := range ids {
		if id == "" {
			return nil, fmt.Errorf("lookbook_card_v1 requires %s", names[i])
		}
		if !validID.MatchString(id) {
			return nil, errors.New("invalid product ID")
		}
		p, e := a.s.get("products", id)
		if e != nil {
			return nil, fmt.Errorf("product %s not found", id)
		}
		img, e := a.cardArchivedImage("products", id)
		if e != nil {
			return nil, e
		}
		name, _ := p["name"].(string)
		size, _ := p["size"].(string)
		clr, _ := p["color"].(string)
		products = append(products, lookbookProduct{id, name, size, clr, img})
	}
	if in.HairID == "" {
		in.HairID = slots["hair"]
	}
	var hair image.Image
	hairName := ""
	if in.HairID != "" {
		if !validID.MatchString(in.HairID) {
			return nil, errors.New("invalid hairstyle ID")
		}
		p, e := a.s.get("hairstyles", in.HairID)
		if e != nil {
			return nil, errors.New("hairstyle missing")
		}
		hairName, _ = p["name"].(string)
		hair, e = a.cardArchivedImage("hairstyles", in.HairID)
		if e != nil {
			return nil, e
		}
	}
	portrait, e := a.cardPortrait(in)
	if e != nil {
		return nil, e
	}
	regular, bold, e := cardFonts()
	if e != nil {
		return nil, e
	}
	defer regular.Close()
	defer bold.Close()
	canvas := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	fillCard(canvas, 0, 0, cardW, cardH, col(246, 246, 243))
	fillCard(canvas, 0, 0, cardLeft, cardH, col(219, 221, 219))
	fitCard(canvas, portrait, image.Rect(0, 0, cardLeft, cardH)) // contain, preserve full body
	fillCard(canvas, cardLeft, 0, cardW-cardLeft, cardH, col(242, 240, 233))
	header := func(y int, label string) {
		fillCard(canvas, 804, y, 384, 54, col(49, 53, 50))
		textCard(canvas, bold, 826, y+37, label, col(250, 250, 250), 353)
	}
	header(14, "本次搭配单品")
	for i, p := range products {
		y := 82 + i*246
		fillCard(canvas, 804, y, 384, 233, col(255, 255, 255))
		fillCard(canvas, 816, y+13, 202, 207, col(244, 244, 241))
		fitCard(canvas, p.Image, image.Rect(822, y+19, 1012, y+214))
		textCard(canvas, bold, 1026, y+65, p.ID, col(38, 43, 39), 156)
		wrapCard(canvas, regular, 1026, y+98, 155, 27, 3, p.Name, col(42, 46, 42))
		wrapCard(canvas, regular, 1026, y+186, 155, 24, 2, strings.TrimSpace(p.Color+" · "+p.Size), col(89, 97, 91))
	}
	header(1077, "发型参考")
	fillCard(canvas, 804, 1146, 384, 259, col(255, 255, 255))
	if hair != nil {
		fitCard(canvas, hair, image.Rect(816, 1158, 996, 1345))
		textCard(canvas, bold, 1010, 1225, in.HairID, col(42, 46, 42), 162)
		wrapCard(canvas, regular, 1010, 1260, 162, 27, 3, hairName, col(42, 46, 42))
		textCard(canvas, regular, 816, 1378, "仅参考发型，不代表本人脸", col(98, 105, 98), 366)
	} else {
		textCard(canvas, regular, 828, 1265, "未指定发型参考", col(95, 104, 97), 347)
	}
	header(1427, "穿搭与验收")
	fillCard(canvas, 804, 1493, 384, 291, col(255, 255, 255))
	notes := []string{"人物仅为传入图像，不自动换脸", "商品图和名称来自本地 SQLite", "不添加未选择的包、手表等", "需检查正脸、站姿、服装是否吻合", "本卡不是本人真实实穿证明"}
	for i, line := range notes {
		wrapCard(canvas, regular, 824, 1537+i*45, 352, 26, 2, line, col(68, 74, 69))
	}
	var out bytes.Buffer
	if e = png.Encode(&out, canvas); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
func col(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }
func fillCard(dst *image.RGBA, x, y, w, h int, c color.Color) {
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{C: c}, image.Point{}, draw.Src)
}
func fitCard(dst *image.RGBA, src image.Image, rect image.Rectangle) {
	b := src.Bounds()
	sw, sh, dw, dh := b.Dx(), b.Dy(), rect.Dx(), rect.Dy()
	if sw < 1 || sh < 1 {
		return
	}
	scale := min(float64(dw)/float64(sw), float64(dh)/float64(sh))
	w, h := max(1, int(float64(sw)*scale)), max(1, int(float64(sh)*scale))
	area := image.Rect(rect.Min.X+(dw-w)/2, rect.Min.Y+(dh-h)/2, rect.Min.X+(dw-w)/2+w, rect.Min.Y+(dh-h)/2+h)
	xdraw.CatmullRom.Scale(dst, area, src, b, xdraw.Over, nil)
}
func cardFonts() (font.Face, font.Face, error) {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		candidates = []string{`C:\Windows\Fonts\msyh.ttc`, `C:\Windows\Fonts\simhei.ttf`, `C:\Windows\Fonts\simsun.ttc`}
	case "darwin":
		candidates = []string{"/System/Library/Fonts/PingFang.ttc"}
	default:
		candidates = []string{"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", "/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc", "/usr/share/fonts/truetype/wqy/wqy-microhei.ttc", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"}
	}
	b := goregular.TTF
	for _, p := range candidates {
		if v, e := os.ReadFile(p); e == nil {
			b = v
			break
		}
	}
	coll, e := opentype.ParseCollection(b)
	if e != nil {
		return nil, nil, e
	}
	ft, e := coll.Font(0)
	if e != nil {
		return nil, nil, e
	}
	regular, e := opentype.NewFace(ft, &opentype.FaceOptions{Size: 18, DPI: 72, Hinting: font.HintingFull})
	if e != nil {
		return nil, nil, e
	}
	bold, e := opentype.NewFace(ft, &opentype.FaceOptions{Size: 22, DPI: 72, Hinting: font.HintingFull})
	if e != nil {
		regular.Close()
		return nil, nil, e
	}
	return regular, bold, nil
}
func textCard(dst draw.Image, face font.Face, x, y int, s string, c color.Color, maxW int) {
	var out string
	for _, r := range s {
		candidate := out + string(r)
		if font.MeasureString(face, candidate).Ceil() > maxW {
			break
		}
		out = candidate
	}
	d := font.Drawer{Dst: dst, Src: &image.Uniform{C: c}, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(out)
}
func wrapCard(dst draw.Image, face font.Face, x, y, width, leading, maxLines int, s string, c color.Color) {
	var lines []string
	line := ""
	for _, r := range s {
		next := line + string(r)
		if line != "" && font.MeasureString(face, next).Ceil() > width {
			lines = append(lines, line)
			line = string(r)
		} else {
			line = next
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	for i, v := range lines {
		if i >= maxLines {
			break
		}
		if i == maxLines-1 && len(lines) > maxLines {
			for len(v) > 0 && font.MeasureString(face, v+"…").Ceil() > width {
				rs := []rune(v)
				v = string(rs[:len(rs)-1])
			}
			v += "…"
		}
		textCard(dst, face, x, y+i*leading, v, c, width)
	}
}
func (a *application) serveLookbook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var in lookbookRenderInput
	if e := tryonReadJSON(r, &in); e != nil {
		fail(w, 400, e)
		return
	}
	b, e := a.renderLookbookCard(in)
	if e != nil {
		fail(w, 400, e)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", "inline; filename=wardrobe-lookbook-card-v1.png")
	w.WriteHeader(200)
	_, _ = io.Copy(w, bytes.NewReader(b))
}
