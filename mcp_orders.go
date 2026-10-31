package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WardrobeOrderSummary struct {
	OrderItems          int            `json:"order_items"`
	ByPlatform          map[string]int `json:"by_platform"`
	WearableCandidates  int            `json:"wearable_candidates"`
	WearableByCategory  map[string]int `json:"wearable_by_category"`
	LinkedWardrobe      int            `json:"linked_wardrobe"`
	UnknownOwnership    int            `json:"unknown_ownership"`
	ImagesWithSource    int            `json:"images_with_source"`
	LocalArchivedImages int            `json:"local_archived_images"`
	NeedsImageSource    int            `json:"needs_image_source"`
	Notice              string         `json:"notice"`
}

func (s *store) orderSummary() (WardrobeOrderSummary, error) {
	out := WardrobeOrderSummary{ByPlatform: map[string]int{}, WearableByCategory: map[string]int{}, Notice: "Aggregate local counts only; not proof of complete historical orders or confirmed physical ownership."}
	rows, err := s.orders()
	if err != nil {
		return out, err
	}
	for _, v := range rows {
		out.OrderItems++
		out.ByPlatform[v.Platform]++
		if !v.Wearable {
			continue
		}
		out.WearableCandidates++
		out.WearableByCategory[v.Category]++
		if v.ProductID != "" {
			out.LinkedWardrobe++
		}
		if v.Owned == "unknown" {
			out.UnknownOwnership++
		}
		if v.ImageURL != "" {
			out.ImagesWithSource++
		} else {
			out.NeedsImageSource++
		}
		if v.LocalImage != "" {
			out.LocalArchivedImages++
		}
	}
	return out, nil
}

type WardrobeOrderBatchInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"每批 1..150，默认100；只操作本机数据库中的已导入订单商品"`
}
type WardrobeOrderImageInput struct {
	BatchSize int `json:"batch_size,omitempty" jsonschema:"每批 1..5 张，默认5"`
	Batches   int `json:"batches,omitempty" jsonschema:"本次最多连续 4 批，默认1；只使用订单已核实的图片 CDN 地址"`
}
type WardrobeOrderImageMCPResult struct {
	Attempted     int    `json:"attempted"`
	Cached        int    `json:"cached"`
	Failed        int    `json:"failed"`
	MissingSource int    `json:"missing_source"`
	HasMore       bool   `json:"has_more"`
	Notice        string `json:"notice"`
}

func (a *application) registerOrderMCP(s *mcp.Server) {
	mcp.AddTool(s, tool("get_wardrobe_order_summary", "订单库统计", "只返回本机订单数量、穿搭类别数量、已归档图数量。不返回订单号、个人资料、商品标题、图片字节或文件路径；无写入。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, WardrobeOrderSummary, error) {
			out, err := a.s.orderSummary()
			return nil, out, err
		})
	mcp.AddTool(s, tool("organize_wardrobe_orders", "整理订单为衣橱候选", "明确要求后才能调用：将已导入订单中的鞋服、配饰等自动创建/关联为待确认持有的衣橱商品，保存人工分类和历史记录；仅本机 SQLite，带批量上限。适配器可提供来源特定的历史去重。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in WardrobeOrderBatchInput) (*mcp.CallToolResult, OrderOrganizeResult, error) {
			if in.Limit == 0 {
				in.Limit = 100
			}
			out, err := a.s.organizeWearables(in.Limit)
			return nil, out, err
		})
	mcp.AddTool(s, tool("reconcile_wardrobe_order_duplicates", "归档合并订单重复商品行", "有数据变更：仅用户明确要求时执行。由可选适配器匹配可验证的旧导出/新来源重复商品行，保留旧人工判断和规格，在本机 SQLite 留存被合并行的完整归档。冲突或多规格不合并。只返回计数。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in WardrobeOrderBatchInput) (*mcp.CallToolResult, LegacyReconcileResult, error) {
			if in.Limit == 0 {
				in.Limit = 100
			}
			out, err := privateReconcileOrder(a.s, in.Limit)
			return nil, out, err
		})
	imageTool := tool("cache_wardrobe_order_images", "本地补齐衣橱商品图", "明确要求后调用：只从已启用适配器所允许的图片域名下载已核实的商品图，在本机转为 JPG 并写 SQLite 路径。无Cookie、无访问任意网页/私网、无图像内容返回云端；只返回计数。", false)
	open := true
	imageTool.Annotations.OpenWorldHint = &open
	mcp.AddTool(s, imageTool, func(ctx context.Context, req *mcp.CallToolRequest, in WardrobeOrderImageInput) (*mcp.CallToolResult, WardrobeOrderImageMCPResult, error) {
		out := WardrobeOrderImageMCPResult{Notice: "Downloaded images remain in local Data/images. No product/order IDs or photo bytes are returned to the MCP client."}
		if in.BatchSize == 0 {
			in.BatchSize = 5
		}
		if in.Batches == 0 {
			in.Batches = 1
		}
		if in.BatchSize < 1 || in.BatchSize > 5 || in.Batches < 1 || in.Batches > 4 {
			return nil, out, errors.New("batch_size must be 1..5 and batches 1..4")
		}
		after := ""
		for i := 0; i < in.Batches; i++ {
			result, err := a.s.cacheWardrobeImages(ctx, after, in.BatchSize, wardrobeImageHTTPClient)
			if err != nil {
				return nil, out, fmt.Errorf("image archive batch %d: %w", i+1, err)
			}
			out.Attempted += result.Attempted
			out.Cached += result.Cached
			out.Failed += result.Failed
			out.MissingSource += result.MissingSource
			out.HasMore = result.HasMore
			if !result.HasMore || result.Next == "" {
				break
			}
			after = result.Next
		}
		return nil, out, nil
	})
}
