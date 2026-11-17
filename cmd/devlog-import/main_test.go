package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeRecords 把内容写进临时目录里的记录文件并返回路径。
func writeRecords(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "records.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写记录文件失败: %v", err)
	}
	return path
}

// TestLoadRecordsValid 验证合法的记录文件能被完整读出。
func TestLoadRecordsValid(t *testing.T) {
	path := writeRecords(t, `{
		"board": {"name": "Slate 开发", "slug": "SLATE", "template": "dev"},
		"cards": [
			{"column": "完成", "title": "A", "tags": ["后端"], "priority": 2},
			{"column": "待办", "title": "B"}
		]
	}`)
	rec, err := loadRecords(path)
	if err != nil {
		t.Fatalf("应该读取成功: %v", err)
	}
	if rec.Board.Slug != "SLATE" || len(rec.Cards) != 2 || rec.Cards[0].Priority != 2 {
		t.Fatalf("读出的内容不对: %+v", rec)
	}
}

// TestLoadRecordsRejectsBadInput 验证各种不合法的记录文件都会被拒绝，并说明原因。
func TestLoadRecordsRejectsBadInput(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string
	}{
		"不是 JSON":  {`{`, "解析记录文件"},
		"缺看板标识":    {`{"board":{"name":"x"},"cards":[]}`, "board.slug"},
		"缺看板名称":    {`{"board":{"slug":"X1"},"cards":[]}`, "board.name"},
		"卡片缺标题":    {`{"board":{"name":"x","slug":"X1"},"cards":[{"column":"待办"}]}`, "第 1 张卡片"},
		"卡片缺列":     {`{"board":{"name":"x","slug":"X1"},"cards":[{"title":"a"}]}`, "第 1 张卡片"},
		"标题重复无法去重": {`{"board":{"name":"x","slug":"X1"},"cards":[{"title":"a","column":"待办"},{"title":"a","column":"完成"}]}`, "重复"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadRecords(writeRecords(t, tc.content))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("应该因 %q 被拒绝，实际: %v", tc.want, err)
			}
		})
	}
}

// TestLoadRecordsMissingFile 验证文件不存在时返回错误，而不是 panic。
func TestLoadRecordsMissingFile(t *testing.T) {
	if _, err := loadRecords(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("文件不存在应该返回错误")
	}
}

// TestCardArgsOmitsEmpty 验证空值不会传给服务端，避免 null 或空串被当成"清除"。
func TestCardArgsOmitsEmpty(t *testing.T) {
	args := cardArgs(7, 3, cardRecord{Title: "只有标题"})
	for _, key := range []string{"description", "branch", "url", "due_at", "started_at", "finished_at", "tags", "priority"} {
		if _, ok := args[key]; ok {
			t.Errorf("空的 %s 不应该出现在入参里", key)
		}
	}
	if args["board_id"] != int64(7) || args["column_id"] != int64(3) || args["title"] != "只有标题" {
		t.Fatalf("必填项不对: %+v", args)
	}
}

// TestCardArgsKeepsValues 验证有值的字段都会带上。
func TestCardArgsKeepsValues(t *testing.T) {
	args := cardArgs(1, 2, cardRecord{
		Title: "t", Description: "d", Priority: 3, Tags: []string{"a", "b"},
		StartedAt: "2026-11-17T16:41:00+08:00", FinishedAt: "2026-11-17T16:50:00+08:00", Branch: "master",
	})
	if args["priority"] != 3 || args["description"] != "d" || args["branch"] != "master" {
		t.Fatalf("字段缺失: %+v", args)
	}
	if tags, ok := args["tags"].([]string); !ok || !slices.Equal(tags, []string{"a", "b"}) {
		t.Fatalf("标签不对: %+v", args["tags"])
	}
	if args["started_at"] == nil || args["finished_at"] == nil {
		t.Fatalf("时间缺失: %+v", args)
	}
}
