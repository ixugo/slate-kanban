package kanban_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/stores/blobfs"
)

// pngBytes 返回以 PNG 文件头开头的假图片，类型嗅探只看文件头。
func pngBytes(extra int) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, extra)...)
}

// imageEnv 是带真实目录的图片测试环境：resources 是正式资源目录，uploads 是上传临时目录。
type imageEnv struct {
	*cardEnv
	resources, uploads string
}

// newImageEnv 创建图片测试环境，两个目录都是测试结束自动清理的临时目录。
func newImageEnv(t *testing.T) *imageEnv {
	t.Helper()
	res, up := t.TempDir(), t.TempDir()
	return &imageEnv{cardEnv: newCardEnv(t, kanban.WithResources(blobfs.New(res), blobfs.New(up))), resources: res, uploads: up}
}

// upload 上传一张图片并返回它在 Markdown 里的引用文字，失败就让测试终止。
func (e *imageEnv) upload(t *testing.T) (*kanban.Attachment, string) {
	t.Helper()
	item, err := e.core.SaveAttachment(context.Background(), testActor, e.board.ID, pngBytes(10))
	if err != nil {
		t.Fatal(err)
	}
	return item, "![图](/api/v1/attachments/" + item.Token + ")"
}

// has 判断资源目录里该看板子目录下是否有这张图片的文件。
func (e *imageEnv) has(slug, token string) bool {
	_, err := os.Stat(filepath.Join(e.resources, slug, token+".png"))
	return err == nil
}

// uploaded 判断临时目录里是否还有这张图片的文件。
func (e *imageEnv) uploaded(token string) bool {
	_, err := os.Stat(filepath.Join(e.uploads, token+".png"))
	return err == nil
}

// recorded 判断数据库里是否有这张图片的记录。
func (e *imageEnv) recorded(token string) bool {
	_, err := e.db.Attachment().GetByToken(context.Background(), token)
	return err == nil
}

// TestSaveAttachment 验证上传：类型按内容判断，编号是 32 位小写十六进制，
// 图片先放在临时目录，既没有进资源目录也没有入库，但已经能按编号读回。
func TestSaveAttachment(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	data := pngBytes(100)

	got, err := e.core.SaveAttachment(ctx, testActor, e.board.ID, data)
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if len(got.Token) != 32 || got.ContentType != "image/png" || got.Size != int64(len(data)) || got.BoardID != e.board.ID {
		t.Fatalf("记录不对: %+v", got)
	}
	if !e.uploaded(got.Token) || e.has(e.board.Slug, got.Token) || e.recorded(got.Token) {
		t.Fatal("刚上传的图片应该只在临时目录里")
	}
	read, body, err := e.core.GetAttachment(ctx, got.Token)
	if err != nil || !bytes.Equal(body, data) || read.ContentType != "image/png" {
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
		if _, _, err := e.core.GetAttachment(context.Background(), token); !errors.Is(err, kanban.ErrAttachmentNotFound) {
			t.Errorf("%q：期望不存在，实际 %v", token, err)
		}
	}
}

// TestImageCommittedWithDescription 描述保存时，引用到的临时图片搬进看板的资源子目录并入库，临时文件被清掉，
// 之后仍能读回；没被任何内容引用的图片留在临时目录，不会进项目。
func TestImageCommittedWithDescription(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	used, ref := e.upload(t)
	abandoned, _ := e.upload(t)

	if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "带图", Description: "说明 " + ref}); err != nil {
		t.Fatal(err)
	}
	if !e.has("TST", used.Token) || !e.recorded(used.Token) || e.uploaded(used.Token) {
		t.Fatal("被引用的图片应该进资源目录并入库，临时文件被清掉")
	}
	item, err := e.db.Attachment().GetByToken(ctx, used.Token)
	if err != nil || item.Path != "TST/"+used.Token+".png" || item.BoardID != e.board.ID {
		t.Fatalf("记录不对: %+v %v", item, err)
	}
	if _, body, err := e.core.GetAttachment(ctx, used.Token); err != nil || len(body) != len(pngBytes(10)) {
		t.Fatalf("入库后应该能读回: %v", err)
	}
	if e.has("TST", abandoned.Token) || e.recorded(abandoned.Token) || !e.uploaded(abandoned.Token) {
		t.Fatal("没被引用的图片不应该进项目")
	}
}

// TestImageReleasedWhenReferenceRemoved 描述改短后，不再被引用的图片记录和文件被删掉；
// 仍被评论引用的图片保留，直到最后一处引用也消失。
func TestImageReleasedWhenReferenceRemoved(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	gone, goneRef := e.upload(t)
	kept, keptRef := e.upload(t)
	card, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "改描述", Description: goneRef + "\n" + keptRef})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := e.core.AddComment(ctx, testActor, card.ID, "我也引用 "+keptRef)
	if err != nil {
		t.Fatal(err)
	}

	empty := ""
	if _, err := e.core.UpdateCard(ctx, testActor, card.ID, kanban.UpdateCardInput{Description: &empty}); err != nil {
		t.Fatal(err)
	}
	if e.recorded(gone.Token) || e.has("TST", gone.Token) {
		t.Fatal("没人引用的图片应该连记录带文件一起删除")
	}
	if !e.recorded(kept.Token) || !e.has("TST", kept.Token) {
		t.Fatal("仍被评论引用的图片不能删")
	}
	if err := e.core.DeleteComment(ctx, testActor, comment.ID); err != nil {
		t.Fatal(err)
	}
	if e.recorded(kept.Token) || e.has("TST", kept.Token) {
		t.Fatal("最后一处引用（评论）被删后，图片应该被清理")
	}
}

// TestImageReleasedWhenCardDeleted 删除卡片时，描述和评论里引用的图片一起清理。
func TestImageReleasedWhenCardDeleted(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	inDesc, descRef := e.upload(t)
	inComment, commentRef := e.upload(t)
	card, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "删卡片", Description: descRef})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.AddComment(ctx, testActor, card.ID, commentRef); err != nil {
		t.Fatal(err)
	}
	if !e.recorded(inComment.Token) {
		t.Fatal("评论保存后图片应该入库")
	}
	if err := e.core.DeleteCard(ctx, testActor, card.ID); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{inDesc.Token, inComment.Token} {
		if e.recorded(token) || e.has("TST", token) {
			t.Fatalf("删除卡片后图片 %s 应该被清理", token)
		}
	}
}

// TestImageTokensFromOtherSourcesAreIgnored 引用不存在的编号不报错；别的看板的图片即使被引用后又删掉引用也不会被误删。
func TestImageTokensFromOtherSourcesAreIgnored(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	fake := "![x](/api/v1/attachments/00000000000000000000000000000000)"
	if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "假图", Description: fake}); err != nil {
		t.Fatalf("引用不存在的图片不应该报错: %v", err)
	}

	item, err := e.core.SaveAttachment(ctx, testActor, other.board.ID, pngBytes(10))
	if err != nil {
		t.Fatal(err)
	}
	ref := "![y](/api/v1/attachments/" + item.Token + ")"
	card, err := e.core.CreateCard(ctx, testActor, other.board.ID, kanban.CreateCardInput{Title: "别的看板", Description: ref})
	if err != nil {
		t.Fatal(err)
	}
	mine, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "偷引用", Description: ref})
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	if _, err := e.core.UpdateCard(ctx, testActor, mine.ID, kanban.UpdateCardInput{Description: &empty}); err != nil {
		t.Fatal(err)
	}
	if !e.recorded(item.Token) || !e.has("OTH", item.Token) {
		t.Fatalf("别的看板的图片（卡片 %d 仍在引用）不应该被删", card.ID)
	}
}

// TestDeleteBoardRemovesImageDir 删除看板连资源子目录一起删除，别的看板的子目录保留。
func TestDeleteBoardRemovesImageDir(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	other := newCardEnvOn(t, e.core, e.db, "OTH")
	_, ref := e.upload(t)
	theirs, err := e.core.SaveAttachment(ctx, testActor, other.board.ID, pngBytes(10))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "我的", Description: ref}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.core.CreateCard(ctx, testActor, other.board.ID, kanban.CreateCardInput{Title: "他的", Description: "![z](/api/v1/attachments/" + theirs.Token + ")"}); err != nil {
		t.Fatal(err)
	}

	if err := e.core.DeleteBoard(ctx, testActor, e.board.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.resources, "TST")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("看板的资源子目录应该被删除: %v", err)
	}
	if !e.has("OTH", theirs.Token) {
		t.Fatal("别的看板的图片不应该被删")
	}
}

// TestImageFileLostIsNotFound 数据库有记录但文件被手动删了，读取按"不存在"处理。
func TestImageFileLostIsNotFound(t *testing.T) {
	ctx := context.Background()
	e := newImageEnv(t)
	item, ref := e.upload(t)
	if _, err := e.core.CreateCard(ctx, testActor, e.board.ID, kanban.CreateCardInput{Title: "丢图", Description: ref}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(e.resources, "TST", item.Token+".png")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.core.GetAttachment(ctx, item.Token); !errors.Is(err, kanban.ErrAttachmentNotFound) {
		t.Fatalf("文件丢了应该按不存在处理: %v", err)
	}
}

// TestAttachmentWithoutResources 没配置资源存储时，上传和读取都明确报错，而不是 panic。
func TestAttachmentWithoutResources(t *testing.T) {
	ctx := context.Background()
	e := newCardEnv(t, kanban.WithResources(nil, nil))
	if _, err := e.core.SaveAttachment(ctx, testActor, e.board.ID, pngBytes(10)); !errors.Is(err, kanban.ErrResourcesDisabled) {
		t.Fatalf("上传应该报未配置资源目录: %v", err)
	}
	if _, _, err := e.core.GetAttachment(ctx, "00000000000000000000000000000000"); !errors.Is(err, kanban.ErrResourcesDisabled) {
		t.Fatalf("读取应该报未配置资源目录: %v", err)
	}
}
