package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type workflowEditInput struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Content string `json:"content"`
}
type workflowResetInput struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}
type lookDeleteInput struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}
type tryonEvidenceStatus struct {
	LookID            string `json:"lookId"`
	PhotoPresent      bool   `json:"photoPresent"`
	PhotoSHA256       string `json:"photoSHA256,omitempty"`
	FaceConfirmed     bool   `json:"faceConfirmed"`
	BodyConfirmed     bool   `json:"bodyConfirmed"`
	ConfirmationValid bool   `json:"confirmationValid"`
	Notice            string `json:"notice"`
}

func (a *application) registerExtraMCP(s *mcp.Server) {
	mcp.AddTool(s, tool("get_wardrobe_capabilities", "查看衣橱接口能力", "查询 MCP / Web UI 支持的功能和限制：AI真人试穿尚无受控渲染器，固定画册可直接生成 PNG。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, map[string]any, error) {
			return nil, map[string]any{
				"sourceOfTruth": "local SQLite (not GitHub)",
				"lookbook":      map[string]any{"supported": true, "tool": "compose_wardrobe_lookbook", "layout": "lookbook_card_v1", "width": 1200, "height": 1800, "photoSource": "archived Look JPEG or caller-provided JPEG/PNG Base64", "preservesOriginalProductPhotos": true, "saveByDefault": false},
				"virtualTryon":  map[string]any{"managedRendererAvailable": false, "canPreflight": true, "tool": "preflight_wardrobe_tryon", "blockingReason": "no verified image-input renderer or output validator", "neverClaimIdentityVerified": true},
				"photoEvidence": "confirm_wardrobe_tryon_photo requires explicit user assertions; never confirm based on AI-inferred identity",
				"dataBoundary":  "MCP-returned photos and fields reach the connected AI service; no raw order export or filesystem access",
				"orderWorkflow": map[string]any{"available": true, "summaryTool": "get_wardrobe_order_summary", "organizeTool": "organize_wardrobe_orders", "reconcileTool": "reconcile_wardrobe_order_duplicates", "imageTool": "cache_wardrobe_order_images", "orderHistoryCompletenessGuaranteed": false, "rawOrdersReturnedByOrderTools": false, "imagesReturnedByImageCacheTool": false},
			}, nil
		})
	mcp.AddTool(s, tool("get_wardrobe_tryon_evidence", "查询真人照片核验状态", "只读取照片是否被本人确认、摘要是否仍匹配；无生物识别，不返回原始照片。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, tryonEvidenceStatus, error) {
			o := tryonEvidenceStatus{LookID: in.ID, Notice: "user declaration only; NOT a biometric or image-content verification"}
			if !validID.MatchString(in.ID) {
				return nil, o, errors.New("invalid Look ID")
			}
			sha, e := a.tryonLookPhoto(in.ID)
			if e != nil {
				return nil, o, nil
			}
			o.PhotoPresent = true
			o.PhotoSHA256 = sha
			proof, e := a.s.get("tryon_evidence", "EVID-"+in.ID)
			if e != nil {
				return nil, o, nil
			}
			if proof["photoSHA256"] != sha || proof["confirmedByUser"] != true {
				return nil, o, nil
			}
			o.ConfirmationValid = true
			o.FaceConfirmed = proof["faceConfirmed"] == true
			o.BodyConfirmed = proof["bodyConfirmed"] == true
			return nil, o, nil
		})
	mcp.AddTool(s, tool("confirm_wardrobe_tryon_photo", "确认本人真人参考照片", "仅用户明确确认了真实照片及正脸/全身之后才可调用。调用不证明身份，仅记录用户声明和照片 SHA-256；不得由 AI 自行猜测确认。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in tryonEvidenceInput) (*mcp.CallToolResult, map[string]any, error) {
			if !in.ConfirmRealPhoto {
				return nil, nil, errors.New("explicit real photo confirmation required")
			}
			if e := a.verifyTryonEvidenceWithSource(in, "mcp-user-declaration-not-biometric"); e != nil {
				return nil, nil, e
			}
			return nil, map[string]any{"saved": true, "lookId": in.LookID, "role": in.Role, "notice": "user declaration only; not biometric verification"}, nil
		})
	mcp.AddTool(s, tool("list_wardrobe_workflows", "列出衣橱 Skills", "读取本地 SQLite 版本信息，不携带完整私人规则文本。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, map[string]any, error) {
			out := make([]map[string]any, 0, len(workflowNames))
			for _, name := range workflowNames {
				w, e := a.s.getWorkflow(name)
				if e != nil {
					return nil, nil, e
				}
				out = append(out, map[string]any{"name": name, "version": w.Version, "modified": w.Modified, "updatedAt": w.UpdatedAt})
			}
			return nil, map[string]any{"items": out}, nil
		})
	mcp.AddTool(s, tool("get_wardrobe_workflow_source", "获取可编辑 Skill 原文和版本", "从 SQLite override 或内置资源读取原始 Skill，不附加实时个人审美 notes；修改前先读取版本。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in WorkflowInput) (*mcp.CallToolResult, workflowView, error) {
			w, e := a.s.getWorkflow(in.Name)
			return nil, w, e
		})
	mcp.AddTool(s, tool("update_wardrobe_workflow", "更新本地 Skill", "仅用户明确请求编辑时调用，须携带最新 version；写 SQLite，不改 Git 文件。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in workflowEditInput) (*mcp.CallToolResult, workflowView, error) {
			w, e := a.s.updateWorkflow(in.Name, in.Content, in.Version)
			return nil, w, e
		})
	mcp.AddTool(s, tool("reset_wardrobe_workflow", "恢复内置 Skill", "有破坏性：仅用户明确要求恢复，须携带当前 SQLite 版本；删除个人覆盖版，不删除内置资源。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in workflowResetInput) (*mcp.CallToolResult, workflowView, error) {
			w, e := a.s.resetWorkflow(in.Name, in.Version)
			return nil, w, e
		})
	mcp.AddTool(s, tool("delete_wardrobe_look", "删除保存的搭配", "有破坏性：只在用户明确要求删除且已获取当前 version 时调用；不删除商品或照片文件。", false),
		func(ctx context.Context, req *mcp.CallToolRequest, in lookDeleteInput) (*mcp.CallToolResult, map[string]any, error) {
			if !validID.MatchString(in.ID) || in.Version < 1 {
				return nil, nil, errors.New("look ID and positive version required")
			}
			v, e := a.mutateAPI("DELETE", "/api/looks/"+in.ID, map[string]int{"version": in.Version})
			return nil, map[string]any(v), e
		})
	mcp.AddTool(s, tool("compose_wardrobe_lookbook", "生成固定搭配卡 PNG", "生成 1200x1800 的 lookbook_card_v1：左侧单张已提供人物图，右侧仅本地 SQLite 真商品图/文字。传 portraitLookId 或 portraitImageBase64（二选一）。纯合成，不生成真人脸、不自动验证身份、不保存图片。", true),
		func(ctx context.Context, req *mcp.CallToolRequest, in lookbookRenderInput) (*mcp.CallToolResult, any, error) {
			if len(in.PortraitBase64) > 11<<20 {
				return nil, nil, errors.New("portrait too large")
			}
			b, e := a.renderLookbookCard(in)
			if e != nil {
				return nil, nil, fmt.Errorf("lookbook: %w", e)
			}
			note := "lookbook_card_v1 1200x1800 PNG. 左侧人物只是传入的原图，不等于已验证本人试穿；右侧商品图、名称、尺寸均来自本机归档。请人工核验人物、服装和面容。"
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: note}, &mcp.ImageContent{Data: b, MIMEType: "image/png"}}}, nil, nil
		})
}

// The existing local Web UI and the MCP confirmation tool share the exact
// same digest/role checks; only their explicitly recorded declaration source differs.
func (a *application) verifyTryonEvidenceWithSource(in tryonEvidenceInput, source string) error {
	if !strings.HasPrefix(source, "mcp-user-declaration") && source != "local-ui-user-declaration-not-facial-recognition" {
		return errors.New("invalid confirmation source")
	}
	return a.verifyTryonEvidence(in, source)
}
