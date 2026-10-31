package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// A deliberately small XLSX reader: only rows/cells from the first worksheet.
// No formulas/macros, no external files, no workbook modification or Excel execution.
type orderXCell struct {
	Ref    string `xml:"r,attr"`
	Type   string `xml:"t,attr"`
	Value  string `xml:"v"`
	Inline struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"is"`
}
type orderXSheet struct {
	Rows []struct {
		Cells []orderXCell `xml:"c"`
	} `xml:"sheetData>row"`
}
type orderShared struct {
	SI []struct {
		Text string `xml:"t"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
	} `xml:"si"`
}

var xlsxCol = regexp.MustCompile(`^[A-Z]+`)

func orderCellCol(ref string) int {
	m := xlsxCol.FindString(ref)
	n := 0
	for _, c := range m {
		n = n*26 + int(c-'A') + 1
	}
	return n - 1
}
func readXLSXOrderRows(b []byte) ([][]string, error) {
	zr, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return nil, e
	}
	if len(zr.File) > 100 {
		return nil, errors.New("too many XLSX entries")
	}
	var sheet, shared *zip.File
	for _, f := range zr.File {
		if f.UncompressedSize64 > 32<<20 {
			return nil, errors.New("XLSX entry too large")
		}
		if f.Name == "xl/worksheets/sheet1.xml" {
			sheet = f
		}
		if f.Name == "xl/sharedStrings.xml" {
			shared = f
		}
	}
	if sheet == nil {
		return nil, errors.New("missing first worksheet")
	}
	read := func(f *zip.File) ([]byte, error) {
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, 32<<20+1))
	}
	var stringsTable []string
	if shared != nil {
		sb, e := read(shared)
		if e != nil {
			return nil, e
		}
		var table orderShared
		if e = xml.Unmarshal(sb, &table); e != nil {
			return nil, e
		}
		for _, x := range table.SI {
			v := x.Text
			for _, y := range x.Runs {
				v += y.Text
			}
			stringsTable = append(stringsTable, v)
		}
	}
	raw, e := read(sheet)
	if e != nil {
		return nil, e
	}
	var data orderXSheet
	if e = xml.Unmarshal(raw, &data); e != nil {
		return nil, e
	}
	if len(data.Rows) > 20001 {
		return nil, errors.New("too many order rows")
	}
	result := make([][]string, 0, len(data.Rows))
	for _, row := range data.Rows {
		record := map[int]string{}
		max := 0
		for _, c := range row.Cells {
			col := orderCellCol(c.Ref)
			if col < 0 || col > 300 {
				continue
			}
			if col > max {
				max = col
			}
			v := c.Value
			if c.Type == "s" {
				i, e := strconv.Atoi(v)
				if e != nil || i < 0 || i >= len(stringsTable) {
					return nil, errors.New("invalid shared string index")
				}
				v = stringsTable[i]
			}
			if c.Type == "inlineStr" {
				v = c.Inline.Text
				for _, r := range c.Inline.Runs {
					v += r.Text
				}
			}
			record[col] = v
		}
		fields := make([]string, max+1)
		for col, v := range record {
			fields[col] = v
		}
		result = append(result, fields)
	}
	return result, nil
}
func readCSVOrderRows(b []byte) ([][]string, error) {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	r := csv.NewReader(bytes.NewReader(b))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	out := [][]string{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		out = append(out, row)
		if len(out) > 20001 {
			return nil, errors.New("too many order rows")
		}
	}
	return out, nil
}
func normalizeHeader(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))
	s = strings.ToLower(s)
	for _, x := range []string{" ", "\t", "\n", "_", "-", "（", "）", "(", ")", ":", "："} {
		s = strings.ReplaceAll(s, x, "")
	}
	return s
}
func parseOrderTable(platform string, rows [][]string) (OrderImport, error) {
	out := OrderImport{Platform: platform, Items: []OrderItem{}}
	if len(rows) < 2 {
		return out, errors.New("missing order header or items")
	}
	headers := map[string]int{}
	for i, h := range rows[0] {
		k := normalizeHeader(h)
		if k != "" {
			headers[k] = i
		}
	}
	get := func(row []string, aliases ...string) string {
		for _, alias := range aliases {
			if i, ok := headers[normalizeHeader(alias)]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
		}
		return ""
	}
	var previous OrderItem
	for _, row := range rows[1:] {
		id := get(row, "订单号", "订单编号", "订单id", "父订单号", "mainOrderID")
		name := get(row, "商品名称", "商品标题", "商品名", "商品信息", "商品", "sku名称", "产品名称")
		if id == "" && name == "" {
			continue
		}
		// Some tabular exports leave order-level cells empty for additional
		// merchandise rows within the same order. Inherit only from the
		// immediately preceding known order; never fabricate a new order ID.
		continued := id == "" && name != "" && previous.OrderID != ""
		if continued {
			id = previous.OrderID
		}
		if id == "" || name == "" {
			return out, errors.New("required order ID or product name missing in an exported row")
		}

		qty := 1
		if v := get(row, "商品数量", "数量", "购买数量", "qty"); v != "" {
			n, e := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(v), "件"))
			if e != nil || n < 1 {
				return out, errors.New("invalid order item quantity")
			}
			qty = n
		}
		i := OrderItem{Platform: platform, OrderID: id, Name: name, LineID: get(row, "子订单号", "子订单编号", "商品行id", "明细id", "subOrderID"), OrderedAt: get(row, "订单提交时间", "订单时间", "下单时间", "创建时间", "成交时间"), Shop: get(row, "店铺名称", "店铺", "商家名称", "商家"), Spec: get(row, "型号款式", "型号规格", "颜色分类", "规格", "商品规格", "sku"), URL: get(row, "商品链接", "宝贝链接", "链接", "商品url"), ImageURL: get(row, "商品图片", "图片链接", "主图url"), Quantity: qty, Amount: get(row, "商品金额", "单价", "商品价格", "价格"), Paid: get(row, "实付金额", "实付款", "实际支付金额", "成交金额"), OrderStatus: get(row, "订单状态", "交易状态", "状态"), RefundStatus: get(row, "退款状态", "售后状态"), ReturnStatus: get(row, "退货状态")}
		// Excel numeric dates are converted only when the original value is a valid serial.
		if f, e := strconv.ParseFloat(i.OrderedAt, 64); e == nil && f > 25000 && f < 90000 {
			i.OrderedAt = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).Add(time.Duration(f*86400) * time.Second).Format("2006-01-02")
		}
		if continued {
			if i.OrderedAt == "" {
				i.OrderedAt = previous.OrderedAt
			}
			if i.Shop == "" {
				i.Shop = previous.Shop
			}
			if i.OrderStatus == "" {
				i.OrderStatus = previous.OrderStatus
			}
		}
		previous = i
		out.Items = append(out.Items, i)
	}
	if len(out.Items) == 0 {
		return out, errors.New("no order items in file")
	}
	return out, nil
}
func readOrderFile(platform, filename string, data []byte) (OrderImport, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".xlsx":
		rows, e := readXLSXOrderRows(data)
		if e != nil {
			return OrderImport{}, e
		}
		return parseOrderTable(platform, rows)
	case ".csv":
		rows, e := readCSVOrderRows(data)
		if e != nil {
			return OrderImport{}, e
		}
		return parseOrderTable(platform, rows)
	case ".json":
		var in OrderImport
		if e := json.Unmarshal(data, &in); e == nil && len(in.Items) > 0 {
			if in.Platform != "" && in.Platform != platform {
				return OrderImport{}, errors.New("platform mismatch")
			}
			in.Platform = platform
			return in, nil
		}
		var items []OrderItem
		if e := json.Unmarshal(data, &items); e != nil {
			return OrderImport{}, errors.New("JSON must be a normalized items array or {items:[]}")
		}
		return OrderImport{Platform: platform, Items: items}, nil
	default:
		return OrderImport{}, errors.New("only .xlsx, .csv and normalized .json supported")
	}
}
func (a *application) serveOrderFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
		http.Error(w, "multipart upload required", 415)
		return
	}
	if e := r.ParseMultipartForm(16 << 20); e != nil {
		fail(w, 400, errors.New("invalid or oversized upload"))
		return
	}
	platform := r.FormValue("platform")
	if !validPlatform(platform) {
		http.Error(w, "select a valid order source", 400)
		return
	}
	f, header, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, errors.New("file required"))
		return
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, 12<<20+1))
	if e != nil || len(data) > 12<<20 {
		fail(w, 400, errors.New("file too large"))
		return
	}
	in, e := readOrderFile(platform, header.Filename, data)
	if e != nil {
		fail(w, 400, fmt.Errorf("invalid order export: %w", e))
		return
	}
	result, e := a.s.importOrders(in)
	if e != nil {
		fail(w, 400, e)
		return
	}
	jsonOut(w, 200, result)
}
