package kanban_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// setEngineers 以人的身份整体替换看板的工程师名册，返回替换后的名册。
func (e *cardEnv) setEngineers(t *testing.T, in []kanban.EngineerInput) []*kanban.Engineer {
	t.Helper()
	b, err := e.core.UpdateBoard(context.Background(), testActor, e.board.ID, kanban.UpdateBoardInput{Engineers: &in})
	if err != nil {
		t.Fatalf("设置工程师名册失败: %v", err)
	}
	return b.Engineers
}

// twoDevOneReview 招募 2 个全栈和 1 个测试，返回他们（按这个顺序）。
func (e *cardEnv) twoDevOneReview(t *testing.T) (dev1, dev2, rev *kanban.Engineer) {
	t.Helper()
	list := e.setEngineers(t, []kanban.EngineerInput{
		{Name: "全栈一号", Kind: kanban.EngineerDev, Prompt: "做研发"},
		{Name: "全栈二号", Kind: kanban.EngineerDev, Prompt: "做研发"},
		{Name: "审查一号", Kind: kanban.EngineerReview, Prompt: "做审查"},
	})
	return list[0], list[1], list[2]
}

// agentNamed 返回一个指定名字的 agent。
func agentNamed(name string) kanban.Actor {
	return kanban.Actor{Kind: kanban.ActorAgent, Name: name}
}

// TestSeedEngineers 验证新看板按模板起步：研发模板 2 个全栈加 1 个测试，简单模板 2 个全栈，empty 没有。
func TestSeedEngineers(t *testing.T) {
	env := newEnv(t)
	cases := map[string][]string{
		kanban.TemplateDev:    {kanban.EngineerDev, kanban.EngineerDev, kanban.EngineerReview},
		kanban.TemplateSimple: {kanban.EngineerDev, kanban.EngineerDev},
		kanban.TemplateEmpty:  nil,
	}
	for template, kinds := range cases {
		d := mustCreateBoard(t, env.core, kanban.CreateBoardInput{Name: "起步 " + template, Template: template, Locale: "zh-CN"})
		if len(d.Engineers) != len(kinds) {
			t.Fatalf("%s 模板起步工程师数量 = %d，期望 %d", template, len(d.Engineers), len(kinds))
		}
		for i, kind := range kinds {
			if d.Engineers[i].Kind != kind || d.Engineers[i].Name == "" || d.Engineers[i].Prompt == "" {
				t.Fatalf("%s 模板第 %d 位工程师不对: %+v", template, i, d.Engineers[i])
			}
		}
		got, err := env.core.GetBoard(context.Background(), d.ID)
		if err != nil || len(got.Engineers) != len(kinds) {
			t.Fatalf("读回的名册不对: %+v %v", got, err)
		}
	}
}

// TestUpdateEngineers 验证名册的招募、修改、排序和解聘：入参顺序就是名册顺序，没有变化不写事件，变化写 board.updated。
func TestUpdateEngineers(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	dev1, dev2, _ := e.twoDevOneReview(t)
	if last := e.lastEvent(t); last.Type != kanban.EventBoardUpdated || !strings.Contains(last.Payload, "engineers") {
		t.Fatalf("事件不对: %+v", last)
	}
	before := len(e.events(t))
	same := []kanban.EngineerInput{
		{ID: dev1.ID, Name: dev1.Name, Kind: dev1.Kind, Prompt: dev1.Prompt},
		{ID: dev2.ID, Name: dev2.Name, Kind: dev2.Kind, Prompt: dev2.Prompt},
	}
	got := e.setEngineers(t, append(same, kanban.EngineerInput{Name: "审查一号", Kind: kanban.EngineerReview, Prompt: "做审查"}))
	if len(got) != 3 {
		t.Fatalf("名册 = %+v", got)
	}
	if n := len(e.events(t)); n != before+1 {
		t.Fatalf("招募审查后应该多一条事件，实际 %d -> %d", before, n)
	}
	// 同样的名册再提交一次，不算变化。
	again := e.setEngineers(t, []kanban.EngineerInput{
		{ID: got[0].ID, Name: got[0].Name, Kind: got[0].Kind, Prompt: got[0].Prompt},
		{ID: got[1].ID, Name: got[1].Name, Kind: got[1].Kind, Prompt: got[1].Prompt},
		{ID: got[2].ID, Name: got[2].Name, Kind: got[2].Kind, Prompt: got[2].Prompt},
	})
	if len(again) != 3 || len(e.events(t)) != before+1 {
		t.Fatalf("没有变化不应该写事件")
	}
	// 改名并调换顺序。
	swapped := e.setEngineers(t, []kanban.EngineerInput{
		{ID: got[1].ID, Name: "改名了", Kind: got[1].Kind, Prompt: got[1].Prompt},
		{ID: got[0].ID, Name: got[0].Name, Kind: got[0].Kind, Prompt: got[0].Prompt},
	})
	if len(swapped) != 2 || swapped[0].ID != got[1].ID || swapped[0].Name != "改名了" {
		t.Fatalf("改名和排序没有生效: %+v", swapped)
	}
	d, err := e.core.GetBoard(ctx, e.board.ID)
	if err != nil || len(d.Engineers) != 2 || d.Engineers[0].Name != "改名了" {
		t.Fatalf("读回的名册不对: %+v %v", d, err)
	}
}

// TestUpdateEngineersInvalid 验证名册入参的校验：名称、提示词、类型、重名、数量，以及 agent 不能改。
func TestUpdateEngineersInvalid(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	ok := kanban.EngineerInput{Name: "甲", Kind: kanban.EngineerDev, Prompt: "做研发"}
	many := make([]kanban.EngineerInput, 21)
	for i := range many {
		many[i] = kanban.EngineerInput{Name: strings.Repeat("a", i+1), Kind: kanban.EngineerDev, Prompt: "x"}
	}
	cases := map[string][]kanban.EngineerInput{
		"名称为空":    {{Name: " ", Kind: kanban.EngineerDev, Prompt: "x"}},
		"名称太长":    {{Name: strings.Repeat("长", 31), Kind: kanban.EngineerDev, Prompt: "x"}},
		"提示词为空":   {{Name: "乙", Kind: kanban.EngineerDev, Prompt: " "}},
		"提示词太长":   {{Name: "乙", Kind: kanban.EngineerDev, Prompt: strings.Repeat("长", 2001)}},
		"类型不认识":   {{Name: "乙", Kind: "boss", Prompt: "x"}},
		"重名不分大小写": {ok, {Name: "A", Kind: kanban.EngineerDev, Prompt: "x"}, {Name: "a", Kind: kanban.EngineerDev, Prompt: "x"}},
		"超过上限":    many,
		"编号为负":    {{ID: -1, Name: "乙", Kind: kanban.EngineerDev, Prompt: "x"}},
		"头像种子太长":  {{Name: "乙", Kind: kanban.EngineerDev, Prompt: "x", Avatar: strings.Repeat("a", 33)}},
		"头像种子有符号": {{Name: "乙", Kind: kanban.EngineerDev, Prompt: "x", Avatar: "a b<"}},
	}
	for name, in := range cases {
		in := in
		if _, err := e.core.UpdateBoard(ctx, testActor, e.board.ID, kanban.UpdateBoardInput{Engineers: &in}); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s: 期望 ErrInvalidInput，实际 %v", name, err)
		}
	}
	list := []kanban.EngineerInput{ok}
	if _, err := e.core.UpdateBoard(ctx, testAgent, e.board.ID, kanban.UpdateBoardInput{Engineers: &list}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Errorf("agent 改名册应该返回 ErrHumanOnly，实际 %v", err)
	}
}

// TestBindEngineer 验证卡片绑定工程师：只有人能绑，只能绑本看板的全栈工程师，解聘后绑定被清掉。
func TestBindEngineer(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, _, rev := e.twoDevOneReview(t)
	c := e.card(t, colDoing, "任务")

	id := dev1.ID
	got, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{EngineerID: &id})
	if err != nil || got.EngineerID != dev1.ID {
		t.Fatalf("绑定失败: %+v %v", got, err)
	}
	if _, err := e.core.UpdateCard(ctx, testAgent, c.ID, kanban.UpdateCardInput{EngineerID: &id}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 绑定应该返回 ErrHumanOnly，实际 %v", err)
	}
	for name, bad := range map[string]int64{"测试": rev.ID, "不存在": 9999, "为负": -3} {
		if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{EngineerID: &bad}); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Fatalf("绑定%s的工程师应该返回 ErrInvalidInput，实际 %v", name, err)
		}
	}
	if _, err := e.core.CreateCard(ctx, testAgent, e.board.ID, kanban.CreateCardInput{Title: "x", EngineerID: dev1.ID}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 创建时绑定应该返回 ErrHumanOnly，实际 %v", err)
	}
	created, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "带绑定", EngineerID: dev1.ID})
	if err != nil || created.EngineerID != dev1.ID {
		t.Fatalf("创建时绑定失败: %+v %v", created, err)
	}

	e.setEngineers(t, nil)
	if got := e.stored(t, c.ID); got.EngineerID != 0 || got.Version != 3 {
		t.Fatalf("解聘后绑定应该清掉并涨版本: %+v", got)
	}
}

// TestClaimAssignsEngineer 验证领取时的分配：卡片没有绑定就随机挑一位空闲全栈并记到卡片上，
// 占满名额后再领返回 ErrNoIdleEngineer，释放后名额回来；绑定的工程师优先。
func TestClaimAssignsEngineer(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, dev2, _ := e.twoDevOneReview(t)
	a, b, c := e.card(t, colDoing, "甲"), e.card(t, colDoing, "乙"), e.card(t, colDoing, "丙")
	claude := agentNamed("claude")

	da, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{})
	if err != nil || da.EngineerID == 0 || da.ClaimEngineerID != da.EngineerID {
		t.Fatalf("领取应该分配工程师: %+v %v", da, err)
	}
	db, err := e.core.ClaimCard(ctx, claude, b.ID, kanban.ClaimCardInput{})
	if err != nil || db.EngineerID == da.EngineerID || (db.EngineerID != dev1.ID && db.EngineerID != dev2.ID) {
		t.Fatalf("第二张应该落到另一位全栈: %+v %v", db, err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, c.ID, kanban.ClaimCardInput{}); !errors.Is(err, kanban.ErrNoIdleEngineer) {
		t.Fatalf("名额占满应该返回 ErrNoIdleEngineer，实际 %v", err)
	}
	if err := e.core.ReleaseCard(ctx, claude, a.ID); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if got := e.stored(t, a.ID); got.ClaimEngineerID != 0 || got.EngineerID != da.EngineerID {
		t.Fatalf("释放后应该清掉占用、保留分配: %+v", got)
	}
	dc, err := e.core.ClaimCard(ctx, claude, c.ID, kanban.ClaimCardInput{})
	if err != nil || dc.ClaimEngineerID != da.EngineerID {
		t.Fatalf("释放后第三张应该拿到空出来的名额: %+v %v", dc, err)
	}
	// 再领 a：它原来的工程师在做第三张，而 a 绑定着他，所以要排队。
	if _, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{}); !errors.Is(err, kanban.ErrEngineerBusy) {
		t.Fatalf("绑定的工程师在忙应该返回 ErrEngineerBusy，实际 %v", err)
	}
}

// TestClaimEngineerRules 验证指定工程师的规则：类型要对，不能和绑定冲突，同一位不能同时做两张。
func TestClaimEngineerRules(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, dev2, rev := e.twoDevOneReview(t)
	a, b := e.card(t, colDoing, "甲"), e.card(t, colDoing, "乙")
	claude := agentNamed("claude")

	if _, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{EngineerID: rev.ID}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("研发列指定测试应该返回 ErrInvalidInput，实际 %v", err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{EngineerID: 9999}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("指定不存在的工程师应该返回 ErrInvalidInput，实际 %v", err)
	}
	if d, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{EngineerID: dev1.ID}); err != nil || d.ClaimEngineerID != dev1.ID {
		t.Fatalf("指定全栈领取失败: %+v %v", d, err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, b.ID, kanban.ClaimCardInput{EngineerID: dev1.ID}); !errors.Is(err, kanban.ErrEngineerBusy) {
		t.Fatalf("同一位不能同时做两张，实际 %v", err)
	}
	bound := dev2.ID
	if _, err := e.core.UpdateCard(ctx, testActor, b.ID, kanban.UpdateCardInput{EngineerID: &bound}); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, b.ID, kanban.ClaimCardInput{EngineerID: dev1.ID}); !errors.Is(err, kanban.ErrEngineerBound) {
		t.Fatalf("和绑定冲突应该返回 ErrEngineerBound，实际 %v", err)
	}
	if d, err := e.core.ClaimCard(ctx, claude, b.ID, kanban.ClaimCardInput{}); err != nil || d.ClaimEngineerID != dev2.ID {
		t.Fatalf("没指定时应该用绑定的工程师: %+v %v", d, err)
	}
}

// TestClaimHolderByEngineer 验证同名的多个 agent 靠工程师区分：别的工程师不能抢走租约有效的卡，
// 自己用同一位工程师（或没指定）才是续租。
func TestClaimHolderByEngineer(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, dev2, _ := e.twoDevOneReview(t)
	a := e.card(t, colDoing, "甲")
	one, other := agentNamed("agent"), agentNamed("agent")

	if _, err := e.core.ClaimCard(ctx, one, a.ID, kanban.ClaimCardInput{EngineerID: dev1.ID}); err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if _, err := e.core.ClaimCard(ctx, other, a.ID, kanban.ClaimCardInput{EngineerID: dev2.ID}); !errors.Is(err, kanban.ErrEngineerBound) {
		t.Fatalf("同名但换了工程师应该被拒绝，实际 %v", err)
	}
	if _, err := e.core.NextCard(ctx, other, e.board.ID, kanban.NextCardInput{EngineerID: dev2.ID}); err != nil {
		t.Fatalf("另一位工程师取下一张不该出错: %v", err)
	}
	if d, err := e.core.NextCard(ctx, other, e.board.ID, kanban.NextCardInput{EngineerID: dev2.ID}); err != nil || d != nil {
		t.Fatalf("只有一张卡且被别人占着，应该取不到: %+v %v", d, err)
	}
	if d, err := e.core.ClaimCard(ctx, one, a.ID, kanban.ClaimCardInput{}); err != nil || d.ClaimEngineerID != dev1.ID {
		t.Fatalf("没指定视为续租原来的工程师: %+v %v", d, err)
	}
}

// TestClaimReviewEngineer 验证审查列的领取用测试工程师，且不改卡片绑定的全栈工程师；没有测试工程师时不要求。
func TestClaimReviewEngineer(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, _, rev := e.twoDevOneReview(t)
	c := e.card(t, colReview, "待审")
	bound := dev1.ID
	if _, err := e.core.UpdateCard(ctx, testActor, c.ID, kanban.UpdateCardInput{EngineerID: &bound}); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	d, err := e.core.ClaimCard(ctx, agentNamed("reviewer"), c.ID, kanban.ClaimCardInput{})
	if err != nil || d.ClaimEngineerID != rev.ID || d.EngineerID != dev1.ID {
		t.Fatalf("审查应该用测试工程师并保留绑定: %+v %v", d, err)
	}
	e.setEngineers(t, []kanban.EngineerInput{{ID: dev1.ID, Name: dev1.Name, Kind: dev1.Kind, Prompt: dev1.Prompt}})
	if err := e.core.ReleaseCard(ctx, agentNamed("reviewer"), c.ID); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if d, err := e.core.ClaimCard(ctx, agentNamed("reviewer"), c.ID, kanban.ClaimCardInput{}); err != nil || d.ClaimEngineerID != 0 {
		t.Fatalf("没有测试工程师时不要求工程师: %+v %v", d, err)
	}
}

// TestNextCardWithEngineers 验证取下一张卡：绑定了别的工程师的卡会被跳过，名额满了取不到，
// 并发取卡时不会超过全栈工程师的数量。
func TestNextCardWithEngineers(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	dev1, dev2, _ := e.twoDevOneReview(t)
	a, b := e.card(t, colDoing, "甲"), e.card(t, colDoing, "乙")
	bound := dev2.ID
	if _, err := e.core.UpdateCard(ctx, testActor, a.ID, kanban.UpdateCardInput{EngineerID: &bound}); err != nil {
		t.Fatalf("绑定失败: %v", err)
	}
	d, err := e.core.NextCard(ctx, agentNamed("w1"), e.board.ID, kanban.NextCardInput{EngineerID: dev1.ID})
	if err != nil || d == nil || d.ID != b.ID || d.ClaimEngineerID != dev1.ID {
		t.Fatalf("全栈一号应该跳过绑定给二号的甲，拿到乙: %+v %v", d, err)
	}
	if d, err := e.core.NextCard(ctx, agentNamed("w1"), e.board.ID, kanban.NextCardInput{EngineerID: dev1.ID}); err != nil || d == nil || d.ID != b.ID {
		t.Fatalf("同一位再取应该接着做乙: %+v %v", d, err)
	}
	if d, err := e.core.NextCard(ctx, agentNamed("w2"), e.board.ID, kanban.NextCardInput{}); err != nil || d == nil || d.ID != a.ID || d.ClaimEngineerID != dev2.ID {
		t.Fatalf("没指定工程师时，甲应该按绑定给二号: %+v %v", d, err)
	}
	if d, err := e.core.NextCard(ctx, agentNamed("w3"), e.board.ID, kanban.NextCardInput{}); err != nil || d != nil {
		t.Fatalf("名额满了应该取不到卡: %+v %v", d, err)
	}
}

// TestEngineerAvatar 验证头像种子：招募时留空由服务端生成，修改时留空沿用原来的，显式传入才会换，换了算名册变化。
func TestEngineerAvatar(t *testing.T) {
	e := newCardEnv(t)
	dev1, dev2, rev := e.twoDevOneReview(t)
	if dev1.Avatar == "" || dev2.Avatar == "" || dev1.Avatar == dev2.Avatar {
		t.Fatalf("招募应该各自生成头像种子: %q %q", dev1.Avatar, dev2.Avatar)
	}
	roster := func(avatar string) []kanban.EngineerInput {
		return []kanban.EngineerInput{
			{ID: dev1.ID, Name: dev1.Name, Kind: dev1.Kind, Prompt: dev1.Prompt, Avatar: avatar},
			{ID: dev2.ID, Name: dev2.Name, Kind: dev2.Kind, Prompt: dev2.Prompt},
			{ID: rev.ID, Name: rev.Name, Kind: rev.Kind, Prompt: rev.Prompt},
		}
	}
	before := len(e.events(t))
	keep := e.setEngineers(t, roster(""))
	if keep[0].Avatar != dev1.Avatar || len(e.events(t)) != before {
		t.Fatalf("留空应该沿用原头像且不写事件: %+v", keep[0])
	}
	swap := e.setEngineers(t, roster("newseed_1"))
	if swap[0].Avatar != "newseed_1" || swap[1].Avatar != dev2.Avatar || len(e.events(t)) != before+1 {
		t.Fatalf("显式换头像应该生效并写事件: %+v", swap)
	}
}

// TestDevKindsShareStage 验证前端、后端、UI 和全栈同属研发阶段：都能被卡片绑定、都在研发列领取、互相占名额；
// 测试仍然只干审查阶段，不能绑定，也不能领研发列的卡。
func TestDevKindsShareStage(t *testing.T) {
	ctx := context.Background()
	e := newClaimEnv(t)
	list := e.setEngineers(t, []kanban.EngineerInput{
		{Name: "前端甲", Kind: kanban.EngineerFrontend, Prompt: "做前端"},
		{Name: "后端乙", Kind: kanban.EngineerBackend, Prompt: "做后端"},
		{Name: "界面丙", Kind: kanban.EngineerUI, Prompt: "做界面"},
		{Name: "测试丁", Kind: kanban.EngineerReview, Prompt: "做测试"},
	})
	front, back, ui, tester := list[0], list[1], list[2], list[3]
	claude := agentNamed("claude")

	a, b := e.card(t, colDoing, "甲"), e.card(t, colDoing, "乙")
	for _, bind := range []struct {
		card *kanban.Card
		eng  *kanban.Engineer
	}{{a, front}, {b, ui}} {
		id := bind.eng.ID
		if got, err := e.core.UpdateCard(ctx, testActor, bind.card.ID, kanban.UpdateCardInput{EngineerID: &id}); err != nil || got.EngineerID != id {
			t.Fatalf("应该能绑定%s: %+v %v", bind.eng.Name, got, err)
		}
	}
	tid := tester.ID
	if _, err := e.core.UpdateCard(ctx, testActor, a.ID, kanban.UpdateCardInput{EngineerID: &tid}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("绑定测试应该返回 ErrInvalidInput，实际 %v", err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{EngineerID: tid}); !errors.Is(err, kanban.ErrInvalidInput) {
		t.Fatalf("研发列指定测试应该返回 ErrInvalidInput，实际 %v", err)
	}
	da, err := e.core.ClaimCard(ctx, claude, a.ID, kanban.ClaimCardInput{})
	if err != nil || da.ClaimEngineerID != front.ID {
		t.Fatalf("应该用卡片绑定的前端: %+v %v", da, err)
	}
	c := e.card(t, colDoing, "丙")
	dc, err := e.core.ClaimCard(ctx, claude, c.ID, kanban.ClaimCardInput{EngineerID: back.ID})
	if err != nil || dc.ClaimEngineerID != back.ID {
		t.Fatalf("应该能指定后端领取: %+v %v", dc, err)
	}
	d := e.card(t, colDoing, "丁")
	dd, err := e.core.ClaimCard(ctx, claude, d.ID, kanban.ClaimCardInput{})
	if err != nil || dd.ClaimEngineerID != ui.ID {
		t.Fatalf("前端和后端都占着，随机应该落到 UI: %+v %v", dd, err)
	}
	if _, err := e.core.ClaimCard(ctx, claude, e.card(t, colDoing, "戊").ID, kanban.ClaimCardInput{}); !errors.Is(err, kanban.ErrNoIdleEngineer) {
		t.Fatalf("三位研发都占满应该返回 ErrNoIdleEngineer，实际 %v", err)
	}
}
