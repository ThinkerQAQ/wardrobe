package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestOrderMCPToolsPrivacyAndLocalMutation(t *testing.T) {
	a := testApp(t)
	privateID := "FAKE-ORDER-DO-NOT-RETURN-998877"
	_, e := a.s.importOrders(OrderImport{Platform: "京东", Items: []OrderItem{{OrderID: privateID, LineID: "1", Name: "男士黑色手套 PRIVATE_TITLE", Quantity: 1, ImageURL: "https://assets.example.invalid/n1/fake.jpg"}}})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	ss, e := a.newMCPServer().Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	cl := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, e := cl.Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	listed, e := cs.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]*mcp.Tool{}
	for _, x := range listed.Tools {
		names[x.Name] = x
	}
	for _, key := range []string{"get_wardrobe_order_summary", "organize_wardrobe_orders", "reconcile_wardrobe_order_duplicates", "cache_wardrobe_order_images"} {
		if names[key] == nil {
			t.Fatal("missing order tool", key)
		}
	}
	if names["get_wardrobe_order_summary"].Annotations.ReadOnlyHint != true {
		t.Fatal("read-only order summary marked destructive")
	}
	for _, key := range []string{"organize_wardrobe_orders", "reconcile_wardrobe_order_duplicates", "cache_wardrobe_order_images"} {
		if names[key].Annotations.ReadOnlyHint == true {
			t.Fatal("order mutation marked read-only", key)
		}
	}
	call := func(name string, in any) *mcp.CallToolResult {
		t.Helper()
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: in})
		if e != nil {
			t.Fatal(name, e)
		}
		return r
	}
	out := call("get_wardrobe_order_summary", map[string]any{})
	if out.IsError {
		t.Fatalf("summary failed %+v", out)
	}
	b, _ := json.Marshal(out.StructuredContent)
	if strings.Contains(string(b), privateID) || strings.Contains(string(b), "PRIVATE_TITLE") || !strings.Contains(string(b), "wearable_candidates") {
		t.Fatal("order summary leaked raw item or counts absent")
	}
	r := call("organize_wardrobe_orders", map[string]any{"limit": 10})
	if r.IsError {
		t.Fatalf("organize failed %+v", r)
	}
	b, _ = json.Marshal(r.StructuredContent)
	if strings.Contains(string(b), privateID) {
		t.Fatal("organize leaked order id")
	}
	rows, _ := a.s.orders()
	if rows[0].ProductID == "" {
		t.Fatal("MCP organization did not create pending wardrobe item")
	}
	product, _ := a.s.get("products", rows[0].ProductID)
	if product["owned"] != "待确认" {
		t.Fatal("MCP organization manufactured ownership")
	}
	r = call("reconcile_wardrobe_order_duplicates", map[string]any{"limit": 10})
	if r.IsError {
		t.Fatalf("safe reconcile failed %+v", r)
	}
	r = call("cache_wardrobe_order_images", map[string]any{"batch_size": 6})
	if !r.IsError {
		t.Fatal("unbounded image request accepted")
	}
	// A summary and an unsuccessful order image download never return raw URLs.
	r = call("get_wardrobe_order_summary", map[string]any{})
	b, _ = json.Marshal(r.StructuredContent)
	if strings.Contains(string(b), "360buyimg") || strings.Contains(string(b), privateID) {
		t.Fatal("raw order details escaped from MCP")
	}
}
