package kanbanapi_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBoardExportJSON(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "EXP")
	c := s.newCard(t, b, b.Columns[1].ID, "导出卡")
	s.human(t, http.MethodPost, "/cards/"+id(c.ID)+"/comments", map[string]string{"body": "备注"})
	resp := s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/export", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("导出 JSON 失败: %d %s", resp.Code, resp.Body)
	}
	if ct := resp.Headers.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type 不对: %q", ct)
	}
	cd := resp.Headers.Get("Content-Disposition")
	if !strings.HasPrefix(cd, `attachment; filename="exp-`) || !strings.HasSuffix(cd, `.json"`) {
		t.Fatalf("Content-Disposition 不对: %q", cd)
	}
	var top map[string]json.RawMessage
	resp.JSON(t, &top)
	for _, key := range []string{"format", "version", "exported_at", "seq", "board", "columns", "rules", "cards"} {
		if _, ok := top[key]; !ok {
			t.Fatalf("缺少顶层键 %q", key)
		}
	}
	var format string
	json.Unmarshal(top["format"], &format)
	if format != "slate-board-export" {
		t.Fatalf("format=%q", format)
	}
	var cards []struct {
		Key      string    `json:"key"`
		Column   string    `json:"column"`
		Comments []comment `json:"comments"`
	}
	json.Unmarshal(top["cards"], &cards)
	if len(cards) != 1 || cards[0].Key == "" || cards[0].Column == "" || len(cards[0].Comments) != 1 {
		t.Fatalf("卡片或评论不对: %+v", cards)
	}
}

func TestBoardExportMarkdown(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "MDX")
	s.newCard(t, b, b.Columns[1].ID, "md卡")
	resp := s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/export?format=md", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("导出 md 失败: %d %s", resp.Code, resp.Body)
	}
	if ct := resp.Headers.Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Fatalf("Content-Type 不对: %q", ct)
	}
	body := string(resp.Body)
	if !strings.HasPrefix(body, "# 看板 MDX（MDX）") {
		t.Fatalf("Markdown 标题不对: %.80q", body)
	}
}

func TestBoardExportDefaultFormat(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "DEF")
	resp := s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/export", nil)
	if resp.Code != http.StatusOK || resp.Headers.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("默认应为 JSON: %d %s", resp.Code, resp.Headers.Get("Content-Type"))
	}
}

func TestBoardExportBadFormat(t *testing.T) {
	s := newStack(t)
	b := s.newBoard(t, "BAD")
	wantFail(t, "非法 format", s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/export?format=xml", nil),
		http.StatusBadRequest, "ErrInvalidInput")
}

func TestBoardExportNotFound(t *testing.T) {
	s := newStack(t)
	wantFail(t, "看板不存在", s.human(t, http.MethodGet, "/boards/9999/export", nil),
		http.StatusNotFound, "ErrBoardNotFound")
}

func TestExportAttachmentNameDate(t *testing.T) {
	// 文件名里的日期跟导出当天一致，用包内逻辑间接测：响应头应含今天日期。
	s := newStack(t)
	b := s.newBoard(t, "DT")
	resp := s.human(t, http.MethodGet, "/boards/"+id(b.ID)+"/export?format=md", nil)
	today := time.Now().UTC().Format("20060102") // 服务端用 UTC 时间生成文件名
	cd := resp.Headers.Get("Content-Disposition")
	if !strings.Contains(cd, "dt-"+today+".md") {
		t.Fatalf("文件名应含今天日期: %q", cd)
	}
}
