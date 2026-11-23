package mcpapi_test

import (
	"fmt"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestListBoards 验证看板列表：两个视图都能读，按创建顺序返回，带标识和名称。
func TestListBoards(t *testing.T) {
	s := newStack(t)
	s.seed(t, "ALPHA")
	s.seed(t, "BETA")
	for name, sess := range map[string]func() []map[string]any{
		"agent": func() []map[string]any {
			return items(t, ok(t, s.agent(t, "claude"), "list_boards", nil).Data, "boards")
		},
		"人": func() []map[string]any { return items(t, ok(t, s.human(t), "list_boards", nil).Data, "boards") },
	} {
		boards := sess()
		if len(boards) != 2 || boards[0]["slug"] != "ALPHA" || boards[1]["slug"] != "BETA" || boards[0]["name"] != "看板 ALPHA" {
			t.Errorf("%s：看板列表不对: %v", name, boards)
		}
	}
}

// TestListBoardsEmpty 验证没有看板时返回空数组而不是 null，agent 不用处理 null。
func TestListBoardsEmpty(t *testing.T) {
	s := newStack(t)
	if got := items(t, ok(t, s.agent(t, "claude"), "list_boards", nil).Data, "boards"); len(got) != 0 {
		t.Errorf("没有看板应该返回空数组: %v", got)
	}
}

// TestGetBoard 验证看板详情：列带着说明和给 agent 的指令、领取列标记，规则也在里面；看板不存在是工具错误。
func TestGetBoard(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	sess := s.agent(t, "claude")
	got := ok(t, sess, "get_board", map[string]any{"board_id": b.ID}).Data
	cols := items(t, got, "columns")
	if len(cols) != 5 || len(items(t, got, "rules")) == 0 {
		t.Fatalf("应该有 5 列和默认规则: %v", got)
	}
	doing := cols[2]
	if doing["name"] != "进行中" || !strings.Contains(fmt.Sprint(doing["instruction"]), "finish_card") {
		t.Errorf("进行中列应该是 agent 领取列并带着指令: %v", doing)
	}
	prompt := fmt.Sprint(got["watch_prompt"])
	if !strings.Contains(prompt, "「看板 SLATE」") || strings.Contains(prompt, "%s") || !strings.Contains(prompt, "slate-kanban skill") {
		t.Errorf("watch_prompt 应该带着填好的看板名并指向 skill: %q", prompt)
	}
	// 持续监听靠 wait_for_updates 长连接推送，loop 只检查监听是否还活着；不能回到“本轮不要等待、定时拉取”的旧写法
	if !strings.Contains(prompt, "wait_for_updates(timeout_seconds=3600)") ||
		!strings.Contains(prompt, "wait_for_updates(timeout_seconds=50)") || strings.Contains(prompt, "不要调用 wait_for_updates") ||
		!strings.Contains(prompt, "检查监听是否还活着") {
		t.Errorf("watch_prompt 应该让 agent 用 wait_for_updates 持续监听，loop 只检查监听: %q", prompt)
	}
	// 调度分工：一个终端一个调度 agent，由调度者先建 worktree 和分支再派子 agent
	if !strings.Contains(prompt, "唯一的调度 agent") || !strings.Contains(prompt, "worktree") {
		t.Errorf("watch_prompt 应该写明调度 agent 分工和 worktree: %q", prompt)
	}
	// 并发名额由服务端按名册数好写进提示词：研发模板的起步名册是两位全栈加一位测试
	if !strings.Contains(prompt, "最多同时做 2 个研发任务、同时审 1 个审查任务") || !strings.Contains(prompt, "一个任务新开一个子 agent") {
		t.Errorf("watch_prompt 应该按名册写明并发名额，并要求一个任务一个新子 agent: %q", prompt)
	}
	if !strings.Contains(prompt, "先只读拟定方案") || !strings.Contains(prompt, "comment_card") ||
		!strings.Contains(prompt, "评论成功后") {
		t.Errorf("watch_prompt 应该要求调度者先记录方案再让子 agent 开工: %q", prompt)
	}
	for _, want := range []string{"只能从 get_board 返回的 tags 里挑已有的标签", "不要新建标签", "「任务项」清单"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("watch_prompt 应该提醒标签只用已有的、交接前补任务项，缺少 %q: %q", want, prompt)
		}
	}
	// 待办入口须写在挂起监听之前，守候时不能把进行中为空当成没有任务。
	todoCheck := strings.Index(prompt, "主动检查待办列")
	wait := strings.Index(prompt, "3. 然后一直守候")
	if todoCheck < 0 || wait < 0 || todoCheck >= wait {
		t.Errorf("watch_prompt 应该在开始监听前要求主动检查待办: %q", prompt)
	}
	for _, want := range []string{"禁止在终端触发阻塞式提问", "腾出名额后继续其他可做的任务", "复用或新建配置任务", "不绕过宿主权限"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("watch_prompt 的看板澄清规则缺少 %q: %q", want, prompt)
		}
	}
	missing := call(t, sess, "get_board", map[string]any{"board_id": 9999})
	if !missing.IsError || !strings.HasPrefix(missing.Text, "ErrBoardNotFound:") {
		t.Errorf("看板不存在应该是工具错误: %+v", missing)
	}
}

func TestGetBoardReviewEvidenceWithCustomColumn(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	const projectPrompt = "项目发布前先核对版本记录。"
	const instruction = "由审查工程师验收，交接前核对上线说明。"
	human := s.human(t)
	ok(t, human, "update_board", map[string]any{
		"board_id": b.ID, "description": projectPrompt,
	})
	ok(t, human, "update_column", map[string]any{
		"board_id": b.ID, "column_id": b.review, "instruction": instruction,
	})
	agent := s.agent(t, "claude")
	for _, local := range []string{projectPrompt, instruction} {
		if strings.Contains(agent.InitializeResult().Instructions, local) {
			t.Errorf("全局初始化说明不应包含看板本地提示 %q", local)
		}
	}
	got := ok(t, agent, "get_board", map[string]any{"board_id": b.ID}).Data
	review := items(t, got, "columns")[3]
	if review["instruction"] != instruction {
		t.Errorf("自定义审查列指令应原样返回，实际 %q", review["instruction"])
	}
	prompt := fmt.Sprint(got["watch_prompt"])
	if got["description"] != projectPrompt || !strings.Contains(prompt, "## 项目说明（AGENTS.md）\n\n"+projectPrompt) {
		t.Errorf("项目提示应按看板原样返回: %q", got["description"])
	}
	for _, want := range []string{"所有审查入口", "upload_image", "实际截图", "- [x] 通过", "- [ ] 未通过或未验证", "暂不判通过"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("自定义看板的守候提示词缺少审查证据要求 %q", want)
		}
	}
}

// TestListCardsSummaries 验证任务列表只给摘要：没有描述和评论，有编号、所在列、派生状态；
// 同时支持筛选语法，写错的筛选词是带说明的工具错误。
func TestListCardsSummaries(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	s.card(t, b, b.doing, "做登录")
	s.card(t, b, b.todo, "写文档")
	sess := s.agent(t, "claude")

	all := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID}).Data
	cards := items(t, all, "cards")
	if len(cards) != 2 || all["total"] != float64(2) || all["truncated"] != false {
		t.Fatalf("应该有两个任务: %v", all)
	}
	first := cards[0] // 待办列在进行中列的左边，所以先出现的是"写文档"
	for _, banned := range []string{"description", "comments"} {
		if _, has := first[banned]; has {
			t.Errorf("摘要里不应该有 %s: %v", banned, first)
		}
	}
	if first["title"] != "写文档" || first["key"] != "SLATE-2" || first["column"] != "待办" || first["done"] != false {
		t.Errorf("摘要字段不对: %v", first)
	}

	doing := items(t, ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "query": "col:进行中"}).Data, "cards")
	if len(doing) != 1 || doing[0]["title"] != "做登录" {
		t.Errorf("按列筛选的结果不对: %v", doing)
	}
	bad := call(t, sess, "list_cards", map[string]any{"board_id": b.ID, "query": "is:foo"})
	if !bad.IsError || !strings.Contains(bad.Text, "is:foo") {
		t.Errorf("写错的筛选词应该是带说明的工具错误: %+v", bad)
	}
}

// TestListCardsLimit 验证数量限制：不指定时默认 10 张、最多 30 张，指定了按指定的，并如实告诉调用方被截断了。
func TestListCardsLimit(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	for i := range 55 {
		s.card(t, b, b.todo, fmt.Sprintf("任务 %d", i))
	}
	sess := s.agent(t, "claude")
	def := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID}).Data
	if len(items(t, def, "cards")) != 10 || def["total"] != float64(55) || def["truncated"] != true {
		t.Errorf("默认应该返回 10 张并标记被截断: total=%v truncated=%v", def["total"], def["truncated"])
	}
	most := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": 100}).Data
	if len(items(t, most, "cards")) != 30 || most["truncated"] != true {
		t.Errorf("limit 超过上限应该只返回 30 张: %v", most["truncated"])
	}
	few := ok(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": 3}).Data
	if len(items(t, few, "cards")) != 3 || few["truncated"] != true {
		t.Errorf("limit 为 3 应该返回 3 张: %v", few["truncated"])
	}
	if neg := call(t, sess, "list_cards", map[string]any{"board_id": b.ID, "limit": -1}); !neg.IsError {
		t.Errorf("负数的 limit 应该是工具错误: %+v", neg)
	}
}

// TestGetCard 验证任务详情：全文、评论、所在列的指令、依赖的任务都在里面；任务不存在是工具错误。
func TestGetCard(t *testing.T) {
	s := newStack(t)
	b := s.seed(t, "SLATE")
	dep := s.card(t, b, b.todo, "先做这个")
	card, err := s.core.CreateCard(t.Context(), humanActor, b.ID, kanban.CreateCardInput{
		Title: "做登录", Description: "需要支持扫码", ColumnID: b.doing, WaitsOn: []int64{dep.ID},
	})
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if _, err := s.core.AddComment(t.Context(), humanActor, card.ID, "注意兼容旧版"); err != nil {
		t.Fatalf("写评论失败: %v", err)
	}
	sess := s.agent(t, "claude")
	got := ok(t, sess, "get_card", map[string]any{"card_id": card.ID}).Data
	if got["description"] != "需要支持扫码" || got["waiting"] != true || !strings.Contains(fmt.Sprint(got["column_instruction"]), "finish_card") {
		t.Errorf("详情字段不对: %v", got)
	}
	comments, deps := items(t, got, "comments"), items(t, got, "dependencies")
	if len(comments) != 1 || comments[0]["body"] != "注意兼容旧版" || len(deps) != 1 || deps[0]["title"] != "先做这个" {
		t.Errorf("评论或依赖不对: %v %v", comments, deps)
	}
	missing := call(t, sess, "get_card", map[string]any{"card_id": 9999})
	if !missing.IsError || !strings.HasPrefix(missing.Text, "ErrCardNotFound:") {
		t.Errorf("任务不存在应该是工具错误: %+v", missing)
	}
	if _, ok := got["unread_count"]; ok {
		t.Errorf("get_card 不应带 unread_count: %v", got["unread_count"])
	}
}
