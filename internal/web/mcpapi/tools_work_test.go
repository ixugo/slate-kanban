package mcpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
)

// fails 要求一次工具调用是工具错误，并且错误文本以指定的 reason 开头。
func fails(t *testing.T, got result, reason string) {
	t.Helper()
	if !got.IsError || !strings.HasPrefix(got.Text, reason+":") {
		t.Errorf("应该是 %s 工具错误，实际: %+v", reason, got)
	}
}

// inputProps 取出某个工具入参 Schema 里的字段名集合，用来检查两个视图的同名工具入参不同。
func inputProps(t *testing.T, sess *mcp.ClientSession, tool string) map[string]bool {
	t.Helper()
	for got, err := range sess.Tools(t.Context(), nil) {
		if err != nil {
			t.Fatalf("列出工具失败: %v", err)
		}
		if got.Name != tool {
			continue
		}
		schema, isMap := got.InputSchema.(map[string]any)
		props, hasProps := schema["properties"].(map[string]any)
		if !isMap || !hasProps {
			t.Fatalf("%s 的入参 Schema 不对: %v", tool, got.InputSchema)
		}
		out := make(map[string]bool, len(props))
		for name := range props {
			out[name] = true
		}
		return out
	}
	t.Fatalf("没有找到工具 %s", tool)
	return nil
}

// TestAgentWorkLoop 走完 agent 的一次完整工作：取卡、读指令和全文、写评论、记分支、交接到审查，
// 之后队列空了再取卡，得到 null。
func TestAgentWorkLoop(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")

	next := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data
	detail, _ := next["card"].(map[string]any)
	if detail == nil || detail["id"] != float64(c.ID) || detail["claimed_by"] != "claude" || detail["claim_active"] != true {
		t.Fatalf("应该领到刚建的卡片: %v", next)
	}
	if detail["description"] != "描述：写文档" || detail["column_instruction"] == "" {
		t.Errorf("领到的卡片要带全文和所在列的指令: %v", detail)
	}

	comment := ok(t, sess, "comment_card", map[string]any{"card_id": c.ID, "body": "开始处理"}).Data
	if comment["author"] != "claude" || comment["author_kind"] != "agent" || comment["body"] != "开始处理" {
		t.Errorf("评论的作者和正文不对: %v", comment)
	}
	updated := ok(t, sess, "update_card", map[string]any{"card_id": c.ID, "branch": "feat/docs"}).Data
	if updated["branch"] != "feat/docs" {
		t.Errorf("分支应该写上了: %v", updated)
	}

	done := ok(t, sess, "finish_card", map[string]any{"card_id": c.ID, "summary": "文档写完了", "url": "https://example.com/pr/1"}).Data
	if done["column"] != "审查" || done["claimed_by"] != "" || done["url"] != "https://example.com/pr/1" || done["branch"] != "feat/docs" {
		t.Errorf("交接后应该在审查列、不再被持有、带着链接和分支: %v", done)
	}
	full := ok(t, sess, "get_card", map[string]any{"card_id": c.ID}).Data
	if comments := items(t, full, "comments"); len(comments) != 2 || comments[1]["body"] != "文档写完了" {
		t.Errorf("总结应该写成最后一条评论: %v", full["comments"])
	}

	again := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data
	if card, has := again["card"]; !has || card != nil {
		t.Errorf("队列空了应该返回 card 为 null: %v", again)
	}
}

// TestNextCardSkipsHumanOnly 验证只能由人处理的卡片不会被 agent 领到：取卡跳过它，指定领取被拒绝。
func TestNextCardSkipsHumanOnly(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	mine, err := s.core.CreateCard(t.Context(), humanActor, b.ID, kanban.CreateCardInput{Title: "人来做", ColumnID: b.doing, HumanOnly: true})
	if err != nil {
		t.Fatalf("创建卡片失败: %v", err)
	}
	sess := s.agent(t, "claude")
	if got := ok(t, sess, "next_card", map[string]any{"board_id": b.ID}).Data; got["card"] != nil {
		t.Errorf("只能由人处理的卡片不应该被领走: %v", got)
	}
	fails(t, call(t, sess, "claim_card", map[string]any{"card_id": mine.ID}), "ErrCardHumanOnly")
}

// TestClaimAndRelease 验证领取和释放：别的 agent 不能抢走有效的领取；释放后可以被别人领；
// 人可以收回任何一张；释放是幂等的。
func TestClaimAndRelease(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	first, second, human := s.agent(t, "claude"), s.agent(t, "codex"), s.human(t)
	cardArg := map[string]any{"card_id": c.ID}

	if got := ok(t, first, "claim_card", map[string]any{"card_id": c.ID, "lease_minutes": 30}).Data; got["claimed_by"] != "claude" {
		t.Fatalf("应该领取成功: %v", got)
	}
	fails(t, call(t, second, "claim_card", cardArg), "ErrCardClaimed")
	fails(t, call(t, second, "release_card", cardArg), "ErrNotHolder")

	if got := ok(t, first, "release_card", cardArg).Data; got["ok"] != true {
		t.Errorf("释放应该返回 ok: %v", got)
	}
	if got := ok(t, second, "claim_card", cardArg).Data; got["claimed_by"] != "codex" {
		t.Errorf("释放之后别人应该能领: %v", got)
	}
	ok(t, human, "release_card", cardArg)
	if got := ok(t, human, "get_card", cardArg).Data; got["claimed_by"] != "" {
		t.Errorf("人应该能收回领取: %v", got)
	}
	ok(t, human, "release_card", cardArg) // 已经没有领取，再释放也成功
}

// TestAgentLimits 验证 agent 的限制原样透传：Core 怎么拒绝，工具就怎么报错，
// 不会因为换了入口而放宽。
func TestAgentLimits(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	other := s.card(t, b, b.todo, "没人领的卡")
	sess := s.agent(t, "claude")
	cardArg := func(extra map[string]any) map[string]any {
		args := map[string]any{"card_id": other.ID}
		for k, v := range extra {
			args[k] = v
		}
		return args
	}

	fails(t, call(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": "插队", "column_id": b.doing}), "ErrAgentPickupColumn")
	fails(t, call(t, sess, "claim_card", cardArg(nil)), "ErrColumnNotPickup")
	fails(t, call(t, sess, "update_card", cardArg(map[string]any{"title": "改名"})), "ErrNotHolder")
	fails(t, call(t, sess, "move_card", cardArg(map[string]any{"column_id": b.review})), "ErrNotHolder")
	fails(t, call(t, sess, "finish_card", cardArg(map[string]any{"summary": "做完了"})), "ErrNotHolder")
	fails(t, call(t, sess, "claim_card", map[string]any{"card_id": 9999}), "ErrCardNotFound")

	ok(t, sess, "comment_card", cardArg(map[string]any{"body": "没持有也能回复人的问题"}))
}

// TestAgentCannotPlace 验证 agent 持有卡片后，也不能把它放进领取列（换列和交接都一样），只能放进别的列。
// 这里把审查列也设成领取列，才有一个"别的领取列"可以去试。
func TestAgentCannotPlace(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	yes := true
	if _, err := s.core.UpdateColumn(t.Context(), humanActor, b.ID, b.review, kanban.UpdateColumnInput{AgentPickup: &yes}); err != nil {
		t.Fatalf("把审查列设成领取列失败: %v", err)
	}
	sess := s.agent(t, "claude")
	ok(t, sess, "claim_card", map[string]any{"card_id": c.ID})

	fails(t, call(t, sess, "move_card", map[string]any{"card_id": c.ID, "column_id": b.review}), "ErrAgentPickupColumn")
	fails(t, call(t, sess, "finish_card", map[string]any{"card_id": c.ID, "summary": "做完了", "column_id": b.review}), "ErrAgentPickupColumn")
	moved := ok(t, sess, "move_card", map[string]any{"card_id": c.ID, "column_id": b.todo}).Data
	if moved["column_id"] != float64(b.todo) || moved["claimed_by"] != "" {
		t.Errorf("移到待办列后应该在待办列、领取被清除: %v", moved)
	}
}

// TestUpdateCardVersionConflict 验证乐观锁：带着过期的版本号修改会被拒绝，带着最新的才行。
func TestUpdateCardVersionConflict(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")
	version := ok(t, sess, "claim_card", map[string]any{"card_id": c.ID}).Data["version"]

	first := ok(t, sess, "update_card", map[string]any{"card_id": c.ID, "title": "新标题", "version": version}).Data
	if first["title"] != "新标题" || first["version"] == version {
		t.Fatalf("带最新版本号应该修改成功并涨版本: %v", first)
	}
	fails(t, call(t, sess, "update_card", map[string]any{"card_id": c.ID, "title": "更新的标题", "version": version}), "ErrCardVersionConflict")
}

// TestSetProjectPath 验证 agent 写项目路径：空的时候可以写，写相同的值是成功，
// 换成别的值被拒绝；空路径和相对路径是输入错误。
func TestSetProjectPath(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	arg := func(id int64, path string) map[string]any {
		return map[string]any{"board_id": id, "project_path": path}
	}

	if got := ok(t, sess, "set_project_path", arg(b.ID, "/work/slate")).Data; got["project_path"] != "/work/slate" {
		t.Fatalf("应该写入路径: %v", got)
	}
	ok(t, sess, "set_project_path", arg(b.ID, "/work/slate"))
	fails(t, call(t, sess, "set_project_path", arg(b.ID, "/work/other")), "ErrProjectPathLocked")
	if got := ok(t, sess, "get_board", map[string]any{"board_id": b.ID}).Data; got["project_path"] != "/work/slate" {
		t.Errorf("看板上应该能读到路径: %v", got)
	}

	empty := s.seed(t, "OTHER")
	fails(t, call(t, sess, "set_project_path", arg(empty.ID, "")), "ErrInvalidInput")
	fails(t, call(t, sess, "set_project_path", arg(empty.ID, "relative/dir")), "ErrInvalidInput")
}

// TestHumanCardTools 验证人的卡片工具带有规划类字段：优先级、仅人处理、依赖、时间都能写，
// 移动到完成列会触发自动化规则写上完成时间。
func TestHumanCardTools(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	dep := s.card(t, b, b.todo, "前置")
	human := s.human(t)

	made := ok(t, human, "create_card", map[string]any{
		"board_id": b.ID, "title": "规划", "description": "细节", "column_id": b.todo, "priority": 3,
		"tags": []string{"后端", "api"}, "human_only": true, "waits_on": []int64{dep.ID}, "due_at": "2026-12-01T00:00:00Z",
	}).Data
	if made["priority"] != float64(3) || made["human_only"] != true || fmt.Sprint(made["tags"]) != "[后端 api]" ||
		fmt.Sprint(made["waits_on"]) != fmt.Sprint([]float64{float64(dep.ID)}) || !strings.HasPrefix(fmt.Sprint(made["due_at"]), "2026-12-01") {
		t.Fatalf("规划类字段应该都写上了: %v", made)
	}

	id := made["id"]
	updated := ok(t, human, "update_card", map[string]any{
		"card_id": id, "priority": 1, "human_only": false, "started_at": "2026-11-17T09:00:00Z", "due_at": "",
	}).Data
	if updated["priority"] != float64(1) || updated["human_only"] != false || updated["due_at"] != nil ||
		!strings.HasPrefix(fmt.Sprint(updated["started_at"]), "2026-11-17") {
		t.Errorf("修改规划类字段应该生效，空字符串清除时间: %v", updated)
	}

	moved := ok(t, human, "move_card", map[string]any{"card_id": id, "column_id": b.done}).Data
	if moved["column_id"] != float64(b.done) || moved["finished_at"] == nil {
		t.Errorf("移到完成列应该触发规则写上完成时间: %v", moved)
	}
}

// TestViewsHaveDifferentInputs 验证同名工具在两个视图里入参不同：agent 的创建和修改卡片没有规划类字段，
// 直接传会被 Schema 拒绝；人的有。
func TestViewsHaveDifferentInputs(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	agent, human := s.agent(t, "claude"), s.human(t)
	planning := []string{"priority", "human_only", "waits_on", "due_at", "started_at", "finished_at"}

	for _, tool := range []string{"create_card", "update_card"} {
		agentProps, humanProps := inputProps(t, agent, tool), inputProps(t, human, tool)
		for _, field := range planning {
			if agentProps[field] || !humanProps[field] {
				t.Errorf("%s 的 %s：agent 有=%v（应该没有），人有=%v（应该有）", tool, field, agentProps[field], humanProps[field])
			}
		}
	}
	got := call(t, agent, "create_card", map[string]any{"board_id": b.ID, "title": "我想要高优先级", "priority": 3})
	if !got.IsError || !strings.Contains(got.Text, "priority") {
		t.Errorf("agent 传规划类字段应该被 Schema 拒绝，并说明是哪个字段: %+v", got)
	}
}

// TestWriteResultsAreBrief 验证写入类工具的结果不回显描述：描述最长两万字，agent 刚刚自己写的内容不用再读一遍，
// 但版本号要带上，方便下次修改时做冲突检查。
func TestWriteResultsAreBrief(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	long := strings.Repeat("长描述", 2000)

	res := ok(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": "带长描述", "description": long, "tags": []string{"a"}})
	if _, has := res.Data["description"]; has || strings.Contains(res.Text, "长描述长描述") {
		t.Errorf("结果不应该回显描述: %.200s", res.Text)
	}
	if res.Data["version"] == nil || res.Data["key"] != "SLATE-1" || res.Data["column_id"] != float64(b.idea) {
		t.Errorf("结果应该有版本号、编号，并默认放进第一列: %v", res.Data)
	}
	detail := ok(t, sess, "get_card", map[string]any{"card_id": res.Data["id"]}).Data
	if detail["description"] != long || detail["created_by"] != "claude" {
		t.Errorf("描述应该完整保存，创建者是 agent: %.200v", detail)
	}
}

// TestWriteInputsAreChecked 验证写入类工具的输入由 Core 校验，错误是带说明的工具错误。
func TestWriteInputsAreChecked(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	c := s.card(t, b, b.doing, "写文档")
	sess := s.agent(t, "claude")
	ok(t, sess, "claim_card", map[string]any{"card_id": c.ID})

	cases := map[string]result{
		"空标题":     call(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": "  "}),
		"标题太长":    call(t, sess, "create_card", map[string]any{"board_id": b.ID, "title": strings.Repeat("长", 201)}),
		"链接协议不对":  call(t, sess, "update_card", map[string]any{"card_id": c.ID, "url": "javascript:alert(1)"}),
		"评论为空":    call(t, sess, "comment_card", map[string]any{"card_id": c.ID, "body": " "}),
		"总结为空":    call(t, sess, "finish_card", map[string]any{"card_id": c.ID, "summary": ""}),
		"租约太长":    call(t, sess, "claim_card", map[string]any{"card_id": c.ID, "lease_minutes": 100000}),
		"租约是负数":   call(t, sess, "next_card", map[string]any{"board_id": b.ID, "lease_minutes": -1}),
		"移到不存在的列": call(t, sess, "move_card", map[string]any{"card_id": c.ID, "column_id": 9999}),
	}
	for name, got := range cases {
		if !got.IsError || !strings.HasPrefix(got.Text, "ErrInvalidInput:") {
			t.Errorf("%s：应该是 ErrInvalidInput 工具错误，实际 %+v", name, got)
		}
	}
}
