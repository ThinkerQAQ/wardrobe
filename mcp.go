package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SearchInput struct {
	Query         string `json:"query,omitempty" jsonschema:"关键词，可按编号、名称、颜色、尺码搜索"`
	Category      string `json:"category,omitempty" jsonschema:"分类精确匹配，如外套、裤装、鞋靴"`
	MinRating     int    `json:"min_rating,omitempty" jsonschema:"最低星级0到5；原表评分不等于本人明确喜好"`
	UserRatedOnly bool   `json:"user_rated_only,omitempty" jsonschema:"仅本人在应用中确认的评分"`
	Owned         string `json:"owned,omitempty" jsonschema:"持有状态精确匹配"`
	Fit           string `json:"fit,omitempty" jsonschema:"合身反馈精确匹配"`
	Offset        int    `json:"offset,omitempty"`
	Limit         int    `json:"limit,omitempty" jsonschema:"默认20，最大100"`
}
type IDInput struct {
	ID string `json:"id" jsonschema:"稳定商品或搭配编号"`
}
type ImageInput struct {
	ID   string `json:"id"`
	Kind string `json:"kind" jsonschema:"products / looks / hairstyles；仅读取记录已归档的本地图片"`
}
type LooksInput struct {
	ProductID string `json:"product_id,omitempty"`
}
type LookInput struct {
	ID      string            `json:"id,omitempty" jsonschema:"修改已有搭配时必填；留空新建"`
	Version int               `json:"version,omitempty" jsonschema:"修改时必须使用最近读取的version"`
	Name    string            `json:"name"`
	Scene   string            `json:"scene,omitempty"`
	Rating  int               `json:"rating" jsonschema:"0到5，0表示未评价，不替用户推断喜好"`
	Comment string            `json:"comment,omitempty"`
	Slots   map[string]string `json:"slots" jsonschema:"部位到商品编号；outer mid inner pants socks shoes glasses hat gloves necklace watch belt scarf bag tie accessory1 accessory2 other；空字符串清空"`
}
type ReviewInput struct {
	ID      string  `json:"id"`
	Version int     `json:"version"`
	Rating  int     `json:"rating" jsonschema:"用户明确给出的0到5星"`
	Owned   *string `json:"owned,omitempty"`
	Fit     *string `json:"fit,omitempty"`
	Comment *string `json:"comment,omitempty"`
}
type BodyPatchInput struct {
	ID           string             `json:"id" jsonschema:"固定为 BODY"`
	Version      int                `json:"version" jsonschema:"get_wardrobe_body 返回的当前版本"`
	Measurements map[string]float64 `json:"measurements" jsonschema:"只提交用户明确重新测量的字段，单位厘米；旧数值保留在 history"`
}
type WorkflowInput struct {
	Name string `json:"name" jsonschema:"wardrobe-import / wardrobe-outfits / wardrobe-profile / wardrobe-aesthetics"`
}
type WorkflowOutput struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
type ItemsOutput struct {
	Items      []record `json:"items"`
	Total      int      `json:"total"`
	NextOffset *int     `json:"next_offset,omitempty"`
}
type RecordOutput struct {
	Record record `json:"record"`
}

func tool(name, title, desc string, read bool) *mcp.Tool {
	no := false
	destructive := !read
	return &mcp.Tool{Name: name, Title: title, Description: desc, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: read, OpenWorldHint: &no, DestructiveHint: &destructive}}
}
func (a *application) newMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "wardrobe", Version: "0.9.3"}, &mcp.ServerOptions{Instructions: "主数据在本机SQLite，不上传GitHub。通过云端AI调用MCP时，请求参数与响应中的字段/图片可能传到AI服务；包含真实订单号的导入应优先由本地进程完成。订单自动化：先调用 get_wardrobe_order_summary 查看数量；经用户授权再用 organize_wardrobe_orders 整理候选、reconcile_wardrobe_order_duplicates 安全归档去重、cache_wardrobe_order_images 将静态 CDN 图保存到本机；以上工具只返回汇总计数，不能宣称所有历史订单均被采集。单品手动导入：先查重和核实订单→create_wardrobe_product/update_wardrobe_product→save_wardrobe_image→audit_wardrobe；通过get_wardrobe_workflow(name=wardrobe-import)读取完整流程。搭配使用真实商品且发型hair槽位引用hairstyles记录。身体净尺寸与成衣尺寸分开，get_wardrobe_body显示测量来源和历史日期。评分与合身仅写用户明确反馈，更新须带当前version。Web UI「Skills」可编辑SQLite中的Skill，下一次get_wardrobe_workflow读取最新版本。审美规则请调用get_wardrobe_workflow(name=wardrobe-aesthetics)，该Skill读取最新SQLite notes，不使用旧审美与说明页面；其他操作参见get_wardrobe_workflow(name=wardrobe-outfits或wardrobe-profile)。"})
	mcp.AddTool(s, tool("search_wardrobe", "搜索衣橱", "按关键词、分类、评分与持有/合身状态查询真实衣橱，返回分页摘要。", true), func(ctx context.Context, req *mcp.CallToolRequest, in SearchInput) (*mcp.CallToolResult, ItemsOutput, error) {
		out := ItemsOutput{Items: []record{}}
		if in.Offset < 0 || in.MinRating < 0 || in.MinRating > 5 {
			return nil, out, errors.New("offset must be >=0 and min_rating 0..5")
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		rows, e := a.s.list("products")
		if e != nil {
			return nil, out, e
		}
		q := strings.ToLower(in.Query)
		for _, p := range rows {
			b, _ := json.Marshal(p)
			rating, _ := p["rating"].(float64)
			if (q != "" && !strings.Contains(strings.ToLower(string(b)), q)) || (in.Category != "" && p["category"] != in.Category) || (int(rating) < in.MinRating) || (in.UserRatedOnly && p["ratingSource"] != "user") || (in.Owned != "" && p["owned"] != in.Owned) || (in.Fit != "" && p["fit"] != in.Fit) {
				continue
			}
			if out.Total >= in.Offset && len(out.Items) < limit {
				item := record{}
				for _, k := range []string{"id", "name", "category", "color", "size", "spec", "date", "url", "owned", "fit", "rating", "ratingSource", "comment", "version"} {
					item[k] = p[k]
				}
				item["has_image"] = p["image"] != ""
				out.Items = append(out.Items, item)
			}
			out.Total++
		}
		next := in.Offset + len(out.Items)
		if next < out.Total {
			out.NextOffset = &next
		}
		return nil, out, nil
	})
	mcp.AddTool(s, tool("create_wardrobe_product", "导入衣橱商品", "核实订单并查重后，按唯一 source_key 在本机SQLite原子新增商品；保留购买凭证，评分与持有状态待确认。", false), func(ctx context.Context, req *mcp.CallToolRequest, in CreateProductInput) (*mcp.CallToolResult, RecordOutput, error) {
		p, e := a.s.importProduct(in)
		return nil, RecordOutput{p}, e
	})
	mcp.AddTool(s, tool("update_wardrobe_product", "补充商品资料", "按当前version仅修改明确核实的商品字段，保留所有未提交证据字段与本人评分。", false), func(ctx context.Context, req *mcp.CallToolRequest, in ProductPatchInput) (*mcp.CallToolResult, RecordOutput, error) {
		p, e := a.s.patchProduct(in)
		return nil, RecordOutput{p}, e
	})
	mcp.AddTool(s, tool("save_wardrobe_image", "归档商品主图", "上传经过核验的JPEG或PNG字节Base64，安全转换为本地JPG；同时记录来源和分辨率，禁止从任意本地路径读图。", false), func(ctx context.Context, req *mcp.CallToolRequest, in SaveImageInput) (*mcp.CallToolResult, ImageSaved, error) {
		p, e := a.s.saveProductImage(in)
		return nil, p, e
	})
	mcp.AddTool(s, tool("audit_wardrobe", "衣橱完整性检查", "检查衣橱商品与发型数量、缺图、低清晰度、本轮来源键重复；不修改数据。", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, AuditResult, error) {
		p, e := a.s.auditWardrobe()
		return nil, p, e
	})
	mcp.AddTool(s, tool("get_wardrobe_body", "个人身体数据与成衣参考", "读取用户明确记录的身体净尺寸、成衣合身尺寸、来源时间与历史；不可把旧测量当作实时值。", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, RecordOutput, error) {
		p, e := a.s.get("body", "BODY")
		return nil, RecordOutput{p}, e
	})
	mcp.AddTool(s, tool("update_wardrobe_body", "更新个人身体尺寸", "仅凭用户明确反馈和当前version更新指定厘米尺寸；保留旧数据历史，不修改成衣参考。", false), func(ctx context.Context, req *mcp.CallToolRequest, in BodyPatchInput) (*mcp.CallToolResult, RecordOutput, error) {
		if in.ID != "BODY" {
			return nil, RecordOutput{}, errors.New("only BODY profile can be updated")
		}
		if len(in.Measurements) == 0 {
			return nil, RecordOutput{}, errors.New("no measurements supplied")
		}
		p, e := a.mutateAPI("PUT", "/api/body/BODY", in)
		return nil, RecordOutput{p}, e
	})
	mcp.AddTool(s, tool("list_wardrobe_hairstyles", "发型参考库", "查看本地发型参考图片元数据；非商品，用户未评分的不推断偏好。", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, ItemsOutput, error) {
		p, e := a.s.list("hairstyles")
		return nil, ItemsOutput{Items: p, Total: len(p)}, e
	})
	mcp.AddTool(s, tool("preflight_wardrobe_tryon", "真人试穿预检查", "服务端验证用户确认的真人正脸/全身照片、绑定原图哈希、商品图和配饰白名单。ready只表示可准备受控渲染输入，绝不表示已生成图片；未配置受控渲染器时不可跳过验证。", true), func(ctx context.Context, req *mcp.CallToolRequest, in tryonPreflightInput) (*mcp.CallToolResult, tryonPreflightOutput, error) {
		return nil, a.preflightTryon(in), nil
	})
	mcp.AddTool(s, tool("get_wardrobe_workflow", "读取Wardrobe操作Skill", "连接MCP后可读取最新导入、搭配、审美及身体数据Skill；审美Skill附加SQLite实时规则，不依赖旧页面。", true), func(ctx context.Context, req *mcp.CallToolRequest, in WorkflowInput) (*mcp.CallToolResult, WorkflowOutput, error) {
		out := WorkflowOutput{Name: in.Name}
		if in.Name != "wardrobe-import" && in.Name != "wardrobe-outfits" && in.Name != "wardrobe-profile" && in.Name != "wardrobe-aesthetics" {
			return nil, out, errors.New("unknown workflow")
		}
		if _, e := a.s.getWorkflow(in.Name); e != nil {
			return nil, out, e
		}
		out.Content = a.workflowForMCP(in.Name)
		return nil, out, nil
	})
	mcp.AddTool(s, tool("get_wardrobe_product", "商品详情", "获取商品完整原始规格、尺码证据、评价与version。", true), func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, RecordOutput, error) {
		r, e := a.s.get("products", in.ID)
		return nil, RecordOutput{r}, e
	})
	mcp.AddTool(s, tool("list_wardrobe_looks", "查询搭配", "查看已保存搭配，可筛选包含某件商品的搭配。不将待确认候选当作确定关联。", true), func(ctx context.Context, req *mcp.CallToolRequest, in LooksInput) (*mcp.CallToolResult, ItemsOutput, error) {
		out := ItemsOutput{Items: []record{}}
		rows, e := a.s.list("looks")
		if e != nil {
			return nil, out, e
		}
		for _, r := range rows {
			match := in.ProductID == ""
			if slots, ok := r["slots"].(map[string]any); ok {
				for _, v := range slots {
					if v == in.ProductID {
						match = true
					}
				}
			}
			if match {
				out.Items = append(out.Items, lookForMCP(r))
			}
		}
		out.Total = len(out.Items)
		return nil, out, nil
	})
	mcp.AddTool(s, tool("get_wardrobe_look", "搭配详情", "获取搭配、商品编号和version；实穿照片由专用图片工具读取。", true), func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, RecordOutput, error) {
		r, e := a.s.get("looks", in.ID)
		if e != nil {
			return nil, RecordOutput{}, e
		}
		return nil, RecordOutput{lookForMCP(r)}, nil
	})
	mcp.AddTool(s, tool("get_wardrobe_rules", "审美与尺码规则", "读取用户衣橱审美规则和原始说明；历史统计仅供参考，不重新推断偏好。", true), func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, ItemsOutput, error) {
		rows, e := a.s.list("notes")
		return nil, ItemsOutput{Items: rows, Total: len(rows)}, e
	})
	mcp.AddTool(s, tool("get_wardrobe_image", "查看衣橱图片", "读取商品图、搭配归档照片或发型示意；发型示意不构成本人脸部照片的证据。仅返回指定记录，不访问任意文件。", true), func(ctx context.Context, req *mcp.CallToolRequest, in ImageInput) (*mcp.CallToolResult, any, error) {
		if in.Kind != "products" && in.Kind != "looks" && in.Kind != "hairstyles" {
			return nil, nil, errors.New("kind must be products, looks or hairstyles")
		}
		r, e := a.s.get(in.Kind, in.ID)
		if e != nil {
			return nil, nil, e
		}
		key := "image"
		dir := "images"
		if in.Kind == "looks" {
			key = "photo"
			dir = "photos"
		} else if in.Kind == "hairstyles" {
			dir = "hairstyles"
		}
		imagePath, _ := r[key].(string)
		if imagePath == "" {
			return nil, nil, errors.New("该记录没有已归档图片")
		}
		expected := "/media/" + dir + "/" + in.ID + ".jpg"
		if !validID.MatchString(in.ID) || imagePath != expected {
			return nil, nil, errors.New("invalid media reference")
		}
		b, e := os.ReadFile(filepath.Join(a.s.dir, dir, in.ID+".jpg"))
		if e != nil {
			return nil, nil, e
		}
		if len(b) > 5<<20 {
			return nil, nil, errors.New("image exceeds 5MiB")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: wardrobeImageCaption(in.Kind, in.ID)}, &mcp.ImageContent{Data: b, MIMEType: "image/jpeg"}}}, nil, nil
	})
	mcp.AddTool(s, tool("save_wardrobe_look", "保存搭配", "仅在用户要求保存或修改搭配时使用；更新前读取最新version，slots只能引用真实商品。", false), func(ctx context.Context, req *mcp.CallToolRequest, in LookInput) (*mcp.CallToolResult, RecordOutput, error) {
		method, path := "POST", "/api/looks"
		if in.ID != "" {
			if !validID.MatchString(in.ID) {
				return nil, RecordOutput{}, errors.New("invalid id")
			}
			method = "PUT"
			path += "/" + in.ID
		}
		r, e := a.mutateAPI(method, path, in)
		if e != nil {
			return nil, RecordOutput{}, e
		}
		return nil, RecordOutput{lookForMCP(r)}, nil
	})
	mcp.AddTool(s, tool("update_wardrobe_review", "更新商品评价", "仅按用户明确反馈更新评分、合身与持有状态；必须先读取商品version。", false), func(ctx context.Context, req *mcp.CallToolRequest, in ReviewInput) (*mcp.CallToolResult, RecordOutput, error) {
		if !validID.MatchString(in.ID) {
			return nil, RecordOutput{}, errors.New("invalid id")
		}
		r, e := a.mutateAPI("PUT", "/api/products/"+in.ID, in)
		return nil, RecordOutput{r}, e
	})
	a.registerOrderMCP(s)
	a.registerExtraMCP(s)
	return s
}
func lookForMCP(r record) record {
	out := record{}
	for _, key := range []string{"id", "name", "scene", "rating", "comment", "slots", "candidates", "verification", "unresolved", "version", "updatedAt"} {
		if v, ok := r[key]; ok {
			out[key] = v
		}
	}
	out["has_photo"] = r["photo"] != ""
	return out
}

type apiCapture struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (c *apiCapture) Header() http.Header    { return c.header }
func (c *apiCapture) WriteHeader(status int) { c.status = status }
func (c *apiCapture) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	return c.body.Write(b)
}
func (a *application) mutateAPI(method, path string, input any) (record, error) {
	b, e := json.Marshal(input)
	if e != nil {
		return nil, e
	}
	r, e := http.NewRequest(method, "http://"+a.host+path, bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	c := &apiCapture{header: http.Header{}}
	a.api(c, r)
	if c.status >= 400 {
		return nil, fmt.Errorf("save failed (%d): %s", c.status, c.body.String())
	}
	var out record
	e = json.Unmarshal(c.body.Bytes(), &out)
	return out, e
}
func ensureMCPToken(dir string) (string, error) {
	path := filepath.Join(dir, "mcp-token")
	b, e := os.ReadFile(path)
	if e == nil {
		token := strings.TrimSpace(string(b))
		if len(token) != 64 {
			return "", errors.New("invalid mcp-token file")
		}
		return token, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		return "", e
	}
	token := hex.EncodeToString(raw)
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	_, e = f.WriteString(token)
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	return token, e
}
func mcpHTTP(server *mcp.Server, token string) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "MCP authentication required", 401)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
		handler.ServeHTTP(w, r)
	})
}

func wardrobeImageCaption(kind, id string) string {
	switch kind {
	case "looks":
		return id + " 搭配归档照片；可能是真人实穿，也可能是 AI 预览。请核验原始来源，不能未经确认用作用户身份照片。"
	case "hairstyles":
		return id + " 发型效果参考图；只表示发型样式，图中人物面部不是已核实的用户真人脸照。"
	default:
		return id + " 商品本地归档主图；可能为商家通用图，以购买规格为准。"
	}
}
