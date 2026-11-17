package mcpapi_test

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// idOf 取出结果里的 id 字段。JSON 数字解出来是 float64，这里换回 int64，方便当入参用。
func idOf(t *testing.T, data map[string]any) int64 {
	t.Helper()
	id, isNum := data["id"].(float64)
	if !isNum {
		t.Fatalf("结果里没有数字 id: %v", data)
	}
	return int64(id)
}

// TestBoardTools 验证人管理看板：创建（起步模板和路径）、修改、删除；删除是幂等的，删完读不到。
func TestBoardTools(t *testing.T) {
	s := newStack(t)
	human := s.human(t)

	made := ok(t, human, "create_board", map[string]any{
		"name": "项目甲", "slug": "JIA", "description": "第一个", "project_path": "/work/jia", "locale": "zh-CN",
	}).Data
	id := made["id"]
	if made["slug"] != "JIA" || made["project_path"] != "/work/jia" || len(items(t, made, "columns")) != 5 || len(items(t, made, "rules")) == 0 {
		t.Fatalf("创建结果不对: %v", made)
	}
	if simple := ok(t, human, "create_board", map[string]any{"name": "简单", "template": "simple"}).Data; len(items(t, simple, "columns")) != 3 {
		t.Errorf("simple 模板应该有 3 列: %v", simple)
	}

	updated := ok(t, human, "update_board", map[string]any{"board_id": id, "name": "项目乙", "project_path": "/work/yi"}).Data
	if updated["name"] != "项目乙" || updated["project_path"] != "/work/yi" || updated["description"] != "第一个" {
		t.Errorf("只应该改填写的字段，人可以改已有的路径: %v", updated)
	}
	if cleared := ok(t, human, "update_board", map[string]any{"board_id": id, "project_path": ""}).Data; cleared["project_path"] != "" {
		t.Errorf("传空字符串应该清除路径: %v", cleared)
	}

	if got := ok(t, human, "delete_board", map[string]any{"board_id": id}).Data; got["ok"] != true {
		t.Errorf("删除应该返回 ok: %v", got)
	}
	ok(t, human, "delete_board", map[string]any{"board_id": id})
	fails(t, call(t, human, "get_board", map[string]any{"board_id": id}), "ErrBoardNotFound")
}

// TestBoardToolInputs 验证看板工具的输入校验：标识不合法、模板不认识、名称为空、标识重复、看板不存在
// 都是带说明的工具错误。
func TestBoardToolInputs(t *testing.T) {
	s := newStack(t)
	human := s.human(t)
	ok(t, human, "create_board", map[string]any{"name": "甲", "slug": "JIA"})

	fails(t, call(t, human, "create_board", map[string]any{"name": "乙", "slug": "a"}), "ErrInvalidInput")
	fails(t, call(t, human, "create_board", map[string]any{"name": "乙", "template": "huge"}), "ErrInvalidInput")
	fails(t, call(t, human, "create_board", map[string]any{"name": " "}), "ErrInvalidInput")
	fails(t, call(t, human, "create_board", map[string]any{"name": "乙", "slug": "JIA"}), "ErrBoardSlugTaken")
	fails(t, call(t, human, "update_board", map[string]any{"board_id": 9999, "name": "x"}), "ErrBoardNotFound")
}

// TestColumnTools 验证人管理列：新增、修改（含清除下一列）、重排、删除；
// 重名和过期的顺序被拒绝；有卡片的列要指明移到哪一列；删除是幂等的。
func TestColumnTools(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	human := s.human(t)

	col := ok(t, human, "create_column", map[string]any{
		"board_id": b.ID, "name": "测试中", "instruction": "跑完全部测试", "agent_pickup": true,
	}).Data
	colID := idOf(t, col)
	if col["name"] != "测试中" || col["agent_pickup"] != true || col["instruction"] != "跑完全部测试" {
		t.Fatalf("新列不对: %v", col)
	}
	fails(t, call(t, human, "create_column", map[string]any{"board_id": b.ID, "name": "测试中"}), "ErrColumnNameTaken")

	upd := ok(t, human, "update_column", map[string]any{"board_id": b.ID, "column_id": colID, "name": "验证中", "next_column_id": b.review}).Data
	if upd["name"] != "验证中" || upd["next_column_id"] != float64(b.review) || upd["agent_pickup"] != true {
		t.Errorf("应该只改填写的字段: %v", upd)
	}
	cleared := ok(t, human, "update_column", map[string]any{"board_id": b.ID, "column_id": colID, "next_column_id": 0, "agent_pickup": false}).Data
	if cleared["next_column_id"] != float64(0) || cleared["agent_pickup"] != false {
		t.Errorf("下一列填 0 应该清除，布尔字段填 false 应该生效: %v", cleared)
	}

	order := []int64{colID, b.idea, b.todo, b.doing, b.review, b.done}
	got := items(t, ok(t, human, "reorder_columns", map[string]any{"board_id": b.ID, "ids": order}).Data, "columns")
	if len(got) != 6 || got[0]["id"] != float64(colID) {
		t.Errorf("新列应该排到最前面: %v", got)
	}
	fails(t, call(t, human, "reorder_columns", map[string]any{"board_id": b.ID, "ids": order[:3]}), "ErrColumnOrderStale")

	card := s.card(t, b, b.todo, "待办里的卡")
	fails(t, call(t, human, "delete_column", map[string]any{"board_id": b.ID, "column_id": b.todo}), "ErrColumnNotEmpty")
	ok(t, human, "delete_column", map[string]any{"board_id": b.ID, "column_id": b.todo, "move_to": b.idea})
	if moved := ok(t, human, "get_card", map[string]any{"card_id": card.ID}).Data; moved["column_id"] != float64(b.idea) {
		t.Errorf("删列时卡片应该搬到指定的列: %v", moved)
	}
	ok(t, human, "delete_column", map[string]any{"board_id": b.ID, "column_id": colID})
	ok(t, human, "delete_column", map[string]any{"board_id": b.ID, "column_id": colID})
}

// TestRuleTools 验证人管理自动化规则：用工具建好的规则真的会在卡片进入列时执行；停用后不再执行；
// 修改动作后按新动作执行；配置写错在保存时就被拒绝；删除是幂等的。
func TestRuleTools(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	human := s.human(t)
	tagsOf := func(c *kanban.Card) []any {
		tags, _ := ok(t, human, "get_card", map[string]any{"card_id": c.ID}).Data["tags"].([]any)
		return tags
	}

	rule := ok(t, human, "create_rule", map[string]any{
		"board_id": b.ID, "name": "进入待办加标签", "trigger": "card_entered", "column_id": b.todo,
		"actions": []map[string]any{{"type": "add_tag", "tag": "待排期"}, {"type": "set_priority", "priority": 2}},
	}).Data
	ruleID := idOf(t, rule)
	if rule["enabled"] != true || rule["trigger"] != "card_entered" || len(items(t, rule, "actions")) != 2 {
		t.Fatalf("规则不对: %v", rule)
	}
	first := s.card(t, b, b.todo, "排期")
	if tags := tagsOf(first); len(tags) != 1 || tags[0] != "待排期" {
		t.Errorf("规则应该给新卡片加上标签: %v", tags)
	}

	off := ok(t, human, "update_rule", map[string]any{"rule_id": ruleID, "enabled": false}).Data
	if off["enabled"] != false || len(items(t, off, "actions")) != 2 {
		t.Errorf("只应该改 enabled: %v", off)
	}
	if tags := tagsOf(s.card(t, b, b.todo, "停用后")); len(tags) != 0 {
		t.Errorf("停用的规则不应该执行: %v", tags)
	}

	again := ok(t, human, "update_rule", map[string]any{
		"rule_id": ruleID, "enabled": true, "actions": []map[string]any{{"type": "add_tag", "tag": "新标签"}},
	}).Data
	if len(items(t, again, "actions")) != 1 || again["enabled"] != true {
		t.Fatalf("动作应该被整体替换: %v", again)
	}
	if tags := tagsOf(s.card(t, b, b.todo, "改动作后")); len(tags) != 1 || tags[0] != "新标签" {
		t.Errorf("应该按新动作执行: %v", tags)
	}

	bad := func(args map[string]any) result {
		args["board_id"], args["name"] = b.ID, "错误的规则"
		return call(t, human, "create_rule", args)
	}
	fails(t, bad(map[string]any{"trigger": "card_deleted", "actions": []map[string]any{{"type": "add_tag", "tag": "x"}}}), "ErrInvalidInput")
	fails(t, bad(map[string]any{"trigger": "card_created", "actions": []map[string]any{}}), "ErrInvalidInput")
	fails(t, bad(map[string]any{"trigger": "card_created", "actions": []map[string]any{{"type": "explode"}}}), "ErrInvalidInput")
	fails(t, call(t, human, "update_rule", map[string]any{"rule_id": 9999, "enabled": false}), "ErrRuleNotFound")

	ok(t, human, "delete_rule", map[string]any{"rule_id": ruleID})
	ok(t, human, "delete_rule", map[string]any{"rule_id": ruleID})
}

// TestDeleteCardAndComment 验证人删除卡片和评论：删除后读不到，重复删除也成功。
func TestDeleteCardAndComment(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	human := s.human(t)
	card := s.card(t, b, b.todo, "要删的")
	comment := idOf(t, ok(t, human, "comment_card", map[string]any{"card_id": card.ID, "body": "评论"}).Data)

	ok(t, human, "delete_comment", map[string]any{"comment_id": comment})
	ok(t, human, "delete_comment", map[string]any{"comment_id": comment})
	if got := ok(t, human, "get_card", map[string]any{"card_id": card.ID}).Data; len(items(t, got, "comments")) != 0 {
		t.Errorf("评论应该已经删了: %v", got["comments"])
	}

	if got := ok(t, human, "delete_card", map[string]any{"card_id": card.ID}).Data; got["ok"] != true {
		t.Errorf("删除应该返回 ok: %v", got)
	}
	ok(t, human, "delete_card", map[string]any{"card_id": card.ID})
	fails(t, call(t, human, "get_card", map[string]any{"card_id": card.ID}), "ErrCardNotFound")
}

// TestToolAnnotations 验证给 host 的工具注解：读取类标只读，删除类标破坏性，其余写入类明确标非破坏性，
// 所有工具都标"不和外部世界打交道"。host 靠它们判断哪些调用可以放心自动执行、哪些要先问人。
func TestToolAnnotations(t *testing.T) {
	s := newStack(t)
	readOnly := setOf([]string{"whoami", "list_boards", "get_board", "list_cards", "get_card", "list_events"})
	destroy := setOf([]string{"delete_board", "delete_column", "delete_rule", "delete_card", "delete_comment"})

	sessions := []struct {
		name string
		sess *mcp.ClientSession
	}{{"agent", s.agent(t, "claude")}, {"人", s.human(t)}}
	for _, c := range sessions {
		name := c.name
		for tool, err := range c.sess.Tools(t.Context(), nil) {
			if err != nil {
				t.Fatalf("%s：列出工具失败: %v", name, err)
			}
			a := tool.Annotations
			if a == nil || a.OpenWorldHint == nil || *a.OpenWorldHint {
				t.Errorf("%s：%s 应该标 openWorldHint 为 false: %+v", name, tool.Name, a)
				continue
			}
			switch {
			case readOnly[tool.Name]:
				if !a.ReadOnlyHint {
					t.Errorf("%s：%s 应该标只读", name, tool.Name)
				}
			case destroy[tool.Name]:
				if a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint {
					t.Errorf("%s：%s 应该标破坏性: %+v", name, tool.Name, a)
				}
			default:
				if a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint {
					t.Errorf("%s：%s 应该明确标非破坏性: %+v", name, tool.Name, a)
				}
			}
		}
	}
}

// TestRuleToolDescribesEverything 验证 create_rule 的说明里写全了触发时机、动作类型和模式的名字：
// agent 只能靠这份说明配置规则，名字写错或漏写它就配不对。这里直接用 Core 的常量，改名时测试会提醒同步。
func TestRuleToolDescribesEverything(t *testing.T) {
	s := newStack(t)
	var desc string
	for tool, err := range s.human(t).Tools(t.Context(), nil) {
		if err != nil {
			t.Fatalf("列出工具失败: %v", err)
		}
		if tool.Name == "create_rule" {
			desc = tool.Description
		}
	}
	words := []string{
		kanban.TriggerCardCreated, kanban.TriggerCardEntered, kanban.TriggerCardLeft,
		kanban.ActionSetStartedAt, kanban.ActionSetFinishedAt, kanban.ActionClearStartedAt, kanban.ActionClearFinishedAt,
		kanban.ActionSetDue, kanban.ActionClearDue, kanban.ActionSetPriority, kanban.ActionAddTag, kanban.ActionRemoveTag,
		kanban.ActionSetHumanOnly, kanban.ActionAddComment, kanban.ActionMoveToColumn, kanban.ModeIfEmpty, kanban.ModeAlways,
	}
	for _, word := range words {
		if !strings.Contains(desc, word) {
			t.Errorf("create_rule 的说明里没有 %s", word)
		}
	}
}
