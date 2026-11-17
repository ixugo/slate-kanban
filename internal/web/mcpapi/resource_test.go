package mcpapi_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// readBoard 读取看板资源，返回解析后的 JSON 对象。
func readBoard(t *testing.T, sess *mcp.ClientSession, uri string) map[string]any {
	t.Helper()
	res, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", uri, err)
	}
	if len(res.Contents) != 1 || res.Contents[0].URI != uri || res.Contents[0].MIMEType != "application/json" {
		t.Fatalf("资源内容的地址或类型不对: %+v", res.Contents)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.Contents[0].Text), &out); err != nil {
		t.Fatalf("资源内容不是 JSON 对象: %v\n%s", err, res.Contents[0].Text)
	}
	return out
}

// boardURIOf 生成看板资源的地址。
func boardURIOf(id int64) string { return "slate://boards/" + strconv.FormatInt(id, 10) }

// TestBoardResource 验证看板资源：两个视图都有同一个资源模板；读取得到整板快照，
// 卡片只有摘要（没有描述），带着事件序号 seq，调用方可以接着用 pull_updates 取之后的增量。
func TestBoardResource(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	s.card(t, b, b.todo, "写文档")

	for name, sess := range map[string]*mcp.ClientSession{"agent": s.agent(t, "claude"), "人": s.human(t)} {
		templates, err := sess.ListResourceTemplates(t.Context(), nil)
		if err != nil || len(templates.ResourceTemplates) != 1 || templates.ResourceTemplates[0].URITemplate != "slate://boards/{board_id}" {
			t.Fatalf("%s：资源模板不对: %+v %v", name, templates, err)
		}
		got := readBoard(t, sess, boardURIOf(b.ID))
		cards := items(t, got, "cards")
		if got["slug"] != "SLATE" || len(items(t, got, "columns")) != 5 || len(cards) != 1 || got["total"] != float64(1) || got["truncated"] != false {
			t.Fatalf("%s：快照内容不对: %v", name, got)
		}
		if _, has := cards[0]["description"]; has || cards[0]["title"] != "写文档" || cards[0]["column"] != "待办" {
			t.Errorf("%s：卡片应该是摘要：没有描述，有所在列: %v", name, cards[0])
		}
		if seq, _ := got["seq"].(float64); seq < 1 {
			t.Errorf("%s：快照应该带着事件序号: %v", name, got["seq"])
		}
	}
}

// TestBoardResourceCapsCards 验证快照里的卡片数有上限：卡片多了只给前 200 张的摘要，
// 并如实告诉调用方总数和被截断，调用方再用 list_cards 的筛选去看。
func TestBoardResourceCapsCards(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	for i := range 205 {
		if _, err := s.core.CreateCard(t.Context(), humanActor, b.ID, kanban.CreateCardInput{Title: fmt.Sprintf("卡片 %d", i), ColumnID: b.todo}); err != nil {
			t.Fatalf("创建卡片失败: %v", err)
		}
	}
	got := readBoard(t, s.agent(t, "claude"), boardURIOf(b.ID))
	if len(items(t, got, "cards")) != 200 || got["total"] != float64(205) || got["truncated"] != true {
		t.Errorf("应该只给 200 张，并说明总数和截断: cards=%d total=%v truncated=%v", len(items(t, got, "cards")), got["total"], got["truncated"])
	}
}

// TestBoardResourceRejectsBadURIs 验证地址解析很严格：多余的路径、查询、前导零、符号、别的主机名都按
// "资源不存在"处理，看板不存在也一样；读取和订阅同一套规则。
func TestBoardResourceRejectsBadURIs(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	id := strconv.FormatInt(b.ID, 10)
	bad := []string{
		"slate://boards/", "slate://boards/abc", "slate://boards/0", "slate://boards/0" + id, "slate://boards/-" + id,
		"slate://boards/+" + id, "slate://boards/" + id + "/cards", "slate://boards/" + id + "?x=1", "slate://boards/" + id + "#top",
		"slate://boards/ " + id, "slate://other/" + id, "slate://boards/99999999999999999999", "slate://boards/9999", "http://boards/" + id, "",
	}
	for _, uri := range bad {
		_, err := sess.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
		if !isResourceNotFound(err) {
			t.Errorf("读取 %q 应该返回\"资源不存在\"，实际 %v", uri, err)
		}
		if err := sess.Subscribe(t.Context(), &mcp.SubscribeParams{URI: uri}); !isResourceNotFound(err) {
			t.Errorf("订阅 %q 应该返回\"资源不存在\"，实际 %v", uri, err)
		}
	}
}

// isResourceNotFound 判断错误是不是协议规定的"资源不存在"。
func isResourceNotFound(err error) bool {
	var rpc *jsonrpc.Error
	return errors.As(err, &rpc) && rpc.Code == mcp.CodeResourceNotFound
}
