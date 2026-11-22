package mcpapi_test

import (
	"testing"

	"slate-kanban/internal/core/kanban"
)

// humanActor 是测试里直接调用 Core 时使用的人的身份。
var humanActor = kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}

// board 是测试里的一个看板和它的五个起步列（收件箱、待办、进行中、审查、完成）的编号。
type board struct {
	*kanban.BoardDetail
	idea, todo, doing, review, done int64
}

// seed 用中文起步模板创建一个看板，slug 是看板标识。
func (s *stack) seed(t *testing.T, slug string) board {
	t.Helper()
	d, err := s.core.CreateBoard(t.Context(), humanActor, kanban.CreateBoardInput{Name: "看板 " + slug, Slug: slug, Locale: "zh-CN"})
	if err != nil {
		t.Fatalf("创建看板 %s 失败: %v", slug, err)
	}
	if len(d.Columns) != 5 {
		t.Fatalf("起步模板应该有 5 列，实际 %d", len(d.Columns))
	}
	c := d.Columns
	return board{BoardDetail: d, idea: c[0].ID, todo: c[1].ID, doing: c[2].ID, review: c[3].ID, done: c[4].ID}
}

// card 以人的身份在看板的某一列创建一个任务，描述固定为"描述：标题"。
func (s *stack) card(t *testing.T, b board, column int64, title string) *kanban.Card {
	t.Helper()
	c, err := s.core.CreateCard(t.Context(), humanActor, b.ID, kanban.CreateCardInput{
		Title: title, Description: "描述：" + title, ColumnID: column,
	})
	if err != nil {
		t.Fatalf("创建任务 %s 失败: %v", title, err)
	}
	return c
}

// items 取出结果里某个键对应的对象数组，不是对象数组就让测试终止。
func items(t *testing.T, data map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := data[key].([]any)
	if !ok {
		t.Fatalf("%s 不是数组: %v", key, data[key])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("%s 里的元素不是对象: %v", key, v)
		}
		out = append(out, m)
	}
	return out
}
