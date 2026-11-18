package kanban_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// setLabels 以人的身份整体替换标签库，失败就让测试终止，返回写入后的看板。
func (e *cardEnv) setLabels(t *testing.T, labels []kanban.LabelInput) *kanban.Board {
	t.Helper()
	b, err := e.core.UpdateBoard(context.Background(), testActor, e.board.ID, kanban.UpdateBoardInput{Labels: &labels})
	if err != nil {
		t.Fatalf("修改标签库失败: %v", err)
	}
	return b
}

// tag 给卡片设置标签，失败就让测试终止。
func (e *cardEnv) tag(t *testing.T, c *kanban.Card, tags ...string) {
	t.Helper()
	if _, err := e.core.UpdateCard(context.Background(), testActor, c.ID, kanban.UpdateCardInput{Tags: &tags}); err != nil {
		t.Fatalf("设置标签失败: %v", err)
	}
}

// TestNewBoardHasEmptyLabels 验证没有标签库时读出的是空数组而不是 null。
func TestNewBoardHasEmptyLabels(t *testing.T) {
	e := newCardEnv(t)
	got, err := e.core.GetBoard(context.Background(), e.board.ID)
	if err != nil || got.Labels == nil || len(got.Labels) != 0 {
		t.Fatalf("没有标签库应该是空数组: %+v %v", got.Labels, err)
	}
}

// TestSetLabelsAssignsIDsAndKeepsThemOnReload 验证新增标签由服务端分配编号，再次保存同样的内容不变也不写事件。
func TestSetLabelsAssignsIDsAndKeepsThemOnReload(t *testing.T) {
	e := newCardEnv(t)
	b := e.setLabels(t, []kanban.LabelInput{{Name: "bug", Color: "red-2"}, {Name: "后端"}})
	if len(b.Labels) != 2 || len(b.Labels[0].ID) != 8 || b.Labels[0].ID == b.Labels[1].ID || b.Labels[0].Color != "red-2" {
		t.Fatalf("标签库不对: %+v", b.Labels)
	}
	got, _ := e.core.GetBoard(context.Background(), e.board.ID)
	if !slices.Equal(got.Labels, b.Labels) {
		t.Fatalf("读回的标签库不一致: %+v vs %+v", got.Labels, b.Labels)
	}
	before := len(e.events(t))
	again := []kanban.LabelInput{{ID: b.Labels[0].ID, Name: "bug", Color: "red-2"}, {ID: b.Labels[1].ID, Name: "后端"}}
	e.setLabels(t, again)
	if len(e.events(t)) != before {
		t.Fatalf("没有变化不应该写事件")
	}
}

// TestRenameLabelUpdatesCards 验证改名会同步到所有卡片，卡片版本加一；只改颜色不动卡片。
func TestRenameLabelUpdatesCards(t *testing.T) {
	e := newCardEnv(t)
	b := e.setLabels(t, []kanban.LabelInput{{Name: "bug", Color: "red-2"}})
	id := b.Labels[0].ID
	c := e.card(t, 0, "有标签")
	other := e.card(t, 0, "无关")
	e.tag(t, c, "Bug", "前端")
	version := e.stored(t, c.ID).Version
	otherVersion := e.stored(t, other.ID).Version

	e.setLabels(t, []kanban.LabelInput{{ID: id, Name: "缺陷", Color: "red-2"}})
	got := e.stored(t, c.ID)
	if !slices.Equal(got.Tags, []string{"缺陷", "前端"}) || got.Version != version+1 {
		t.Fatalf("改名没有同步到卡片: %+v", got)
	}
	if e.stored(t, other.ID).Version != otherVersion {
		t.Fatalf("没有这个标签的卡片不应该被改")
	}
	if last := e.lastEvent(t); last.Type != kanban.EventBoardUpdated || !strings.Contains(string(last.Payload), "labels") {
		t.Fatalf("应该写 board.updated 事件并带上 labels: %+v", last)
	}

	e.setLabels(t, []kanban.LabelInput{{ID: id, Name: "缺陷", Color: "green-1"}})
	if e.stored(t, c.ID).Version != version+1 {
		t.Fatalf("只改颜色不应该动卡片")
	}
}

// TestDeleteLabelRemovesFromCards 验证删除标签会从卡片上摘掉；库外的标签不受影响；
// 删掉的同时新库里有同名新标签时，卡片保持原样。
func TestDeleteLabelRemovesFromCards(t *testing.T) {
	e := newCardEnv(t)
	b := e.setLabels(t, []kanban.LabelInput{{Name: "bug"}, {Name: "后端"}})
	c := e.card(t, 0, "卡")
	e.tag(t, c, "bug", "后端", "库外")

	e.setLabels(t, []kanban.LabelInput{{ID: b.Labels[1].ID, Name: "后端"}})
	if got := e.stored(t, c.ID).Tags; !slices.Equal(got, []string{"后端", "库外"}) {
		t.Fatalf("删除后标签应该是 [后端 库外]，实际 %v", got)
	}

	cur, _ := e.core.GetBoard(context.Background(), e.board.ID)
	e.setLabels(t, []kanban.LabelInput{{Name: "后端", Color: "blue-1"}})
	if got := e.stored(t, c.ID).Tags; !slices.Equal(got, []string{"后端", "库外"}) {
		t.Fatalf("同名新标签替换旧标签时卡片应该保持原样，实际 %v", got)
	}
	if cur.Labels[0].ID == e.mustLabels(t)[0].ID {
		t.Fatalf("同名新标签应该有新编号")
	}
}

// mustLabels 读出看板当前的标签库。
func (e *cardEnv) mustLabels(t *testing.T) []kanban.Label {
	t.Helper()
	b, err := e.core.GetBoard(context.Background(), e.board.ID)
	if err != nil {
		t.Fatalf("读取看板失败: %v", err)
	}
	return b.Labels
}

// TestRenameLabelMergesDuplicateTags 验证改名后卡片上出现重复标签（不区分大小写）时只保留一个，互换名称也能正确同步。
func TestRenameLabelMergesDuplicateTags(t *testing.T) {
	e := newCardEnv(t)
	b := e.setLabels(t, []kanban.LabelInput{{Name: "a"}, {Name: "b"}})
	c := e.card(t, 0, "卡")
	e.tag(t, c, "a", "b")
	e.setLabels(t, []kanban.LabelInput{{ID: b.Labels[0].ID, Name: "b"}, {ID: b.Labels[1].ID, Name: "a"}})
	if got := e.stored(t, c.ID).Tags; !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("互换名称后应该是 [b a]，实际 %v", got)
	}
}

// TestSetLabelsRejects 验证非法输入：空名、超长、重复名（忽略大小写）、非法颜色、非法或重复编号、超过上限。
func TestSetLabelsRejects(t *testing.T) {
	e := newCardEnv(t)
	tooMany := make([]kanban.LabelInput, 61)
	for i := range tooMany {
		tooMany[i] = kanban.LabelInput{Name: strings.Repeat("x", 1+i%20) + string(rune('a'+i%26)) + strings.Repeat("y", i/26)}
	}
	cases := map[string][]kanban.LabelInput{
		"空名":    {{Name: "  "}},
		"超长":    {{Name: strings.Repeat("长", 31)}},
		"重复名":   {{Name: "Bug"}, {Name: "bug"}},
		"非法颜色":  {{Name: "a", Color: "teal-2"}},
		"颜色深浅":  {{Name: "a", Color: "red-4"}},
		"非法编号":  {{ID: "xyz", Name: "a"}},
		"重复编号":  {{ID: "0123abcd", Name: "a"}, {ID: "0123abcd", Name: "b"}},
		"超过上限":  tooMany,
		"换行的名称": {{Name: "a\nb"}},
	}
	for name, in := range cases {
		if _, err := e.core.UpdateBoard(context.Background(), testActor, e.board.ID, kanban.UpdateBoardInput{Labels: &in}); !errors.Is(err, kanban.ErrInvalidInput) {
			t.Errorf("%s 应该被拒绝，实际 %v", name, err)
		}
	}
}

// TestSetLabelsOnlyHuman 验证 agent 不能改标签库；人可以。
func TestSetLabelsOnlyHuman(t *testing.T) {
	e := newCardEnv(t)
	labels := []kanban.LabelInput{{Name: "bug"}}
	if _, err := e.core.UpdateBoard(context.Background(), testAgent, e.board.ID, kanban.UpdateBoardInput{Labels: &labels}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 改标签库应该被拒绝，实际 %v", err)
	}
	path := "/tmp"
	if _, err := e.core.UpdateBoard(context.Background(), testAgent, e.board.ID, kanban.UpdateBoardInput{ProjectPath: &path, Labels: &labels}); !errors.Is(err, kanban.ErrHumanOnly) {
		t.Fatalf("agent 同时带标签库和项目路径也应该被拒绝，实际 %v", err)
	}
}

// TestSetLabelsIgnoresForeignIDs 验证入参里带着库里没有的编号时，当作新增并分配新编号。
func TestSetLabelsIgnoresForeignIDs(t *testing.T) {
	e := newCardEnv(t)
	b := e.setLabels(t, []kanban.LabelInput{{ID: "0123abcd", Name: "bug"}})
	if len(b.Labels) != 1 || b.Labels[0].ID == "0123abcd" {
		t.Fatalf("陌生编号应该被换掉: %+v", b.Labels)
	}
}
