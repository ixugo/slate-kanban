package kanban_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"slate-kanban/internal/core/kanban"
)

// pngBytes 返回以 PNG 文件头开头的假图片，类型嗅探只看文件头。
func pngBytes(extra int) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, extra)...)
}

// TestSaveAttachment 验证保存图片：类型按内容判断，编号是 32 位小写十六进制，读回的字节一字不差。
func TestSaveAttachment(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	data := pngBytes(100)

	got, err := e.core.SaveAttachment(ctx, testActor, e.board.ID, data)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if len(got.Token) != 32 || got.ContentType != "image/png" || got.Size != int64(len(data)) || got.BoardID != e.board.ID {
		t.Fatalf("记录不对: %+v", got)
	}
	read, err := e.core.GetAttachment(ctx, got.Token)
	if err != nil || !bytes.Equal(read.Data, data) || read.ContentType != "image/png" {
		t.Fatalf("读回不对: %+v %v", read, err)
	}
	other, err := e.core.SaveAttachment(ctx, testAgent, e.board.ID, data)
	if err != nil || other.Token == got.Token {
		t.Fatalf("agent 也能保存，且编号不能重复: %+v %v", other, err)
	}
}

// TestSaveAttachmentRejects 验证非法输入都被拦住：空内容、超大、非图片（含 SVG 和伪装成图片的文本）、
// 自动化身份、看板不存在。
func TestSaveAttachmentRejects(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	system := kanban.Actor{Kind: kanban.ActorSystem, Name: "automation"}
	cases := map[string]struct {
		actor kanban.Actor
		board int64
		data  []byte
		want  error
	}{
		"空内容":   {testActor, e.board.ID, nil, kanban.ErrInvalidInput},
		"超过上限":  {testActor, e.board.ID, pngBytes(kanban.MaxAttachmentBytes), kanban.ErrAttachmentTooLarge},
		"纯文本":   {testActor, e.board.ID, []byte("hello world"), kanban.ErrInvalidInput},
		"SVG":   {testActor, e.board.ID, []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`), kanban.ErrInvalidInput},
		"HTML":  {testActor, e.board.ID, []byte("<html><script>alert(1)</script></html>"), kanban.ErrInvalidInput},
		"自动化":   {system, e.board.ID, pngBytes(10), kanban.ErrHumanOnly},
		"看板不存在": {testActor, e.board.ID + 999, pngBytes(10), kanban.ErrBoardNotFound},
	}
	for name, c := range cases {
		if _, err := e.core.SaveAttachment(ctx, c.actor, c.board, c.data); !errors.Is(err, c.want) {
			t.Errorf("%s：期望 %v，实际 %v", name, c.want, err)
		}
	}
}

// TestGetAttachmentBadToken 验证编号格式不对或不存在都返回"不存在"，不会变成数据库错误。
func TestGetAttachmentBadToken(t *testing.T) {
	e := newCardEnv(t)
	for _, token := range []string{"", "abc", "../../etc/passwd", "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", "00000000000000000000000000000000"} {
		if _, err := e.core.GetAttachment(context.Background(), token); !errors.Is(err, kanban.ErrAttachmentNotFound) {
			t.Errorf("%q：期望不存在，实际 %v", token, err)
		}
	}
}

// TestDeleteBoardRemovesAttachments 验证删除看板时图片一起删掉，别的看板的图片不受影响。
func TestDeleteBoardRemovesAttachments(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	mine, err := e.core.SaveAttachment(ctx, testActor, e.board.ID, pngBytes(10))
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := e.core.SaveAttachment(ctx, testActor, other.board.ID, pngBytes(10))
	if err != nil {
		t.Fatal(err)
	}

	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := e.core.GetAttachment(ctx, mine.Token); !errors.Is(err, kanban.ErrAttachmentNotFound) {
		t.Fatalf("图片应该被删除: %v", err)
	}
	if _, err := e.core.GetAttachment(ctx, theirs.Token); err != nil {
		t.Fatalf("别的看板的图片不应该被删: %v", err)
	}
}
