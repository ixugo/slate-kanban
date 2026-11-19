package kanban

import (
	"context"
	"log/slog"
	"regexp"
	"slices"

	"github.com/ixugo/goddd/pkg/orm"
)

// attachmentRefRe 匹配 Markdown 里对本服务图片的引用，捕获 32 位的图片编号。
var attachmentRefRe = regexp.MustCompile(`/api/v1/attachments/([0-9a-f]{32})`)

// attachmentTokens 返回文本里引用的全部图片编号，去重并保持出现的顺序。
func attachmentTokens(texts ...string) []string {
	var out []string
	for _, text := range texts {
		for _, m := range attachmentRefRe.FindAllStringSubmatch(text, -1) {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// commitImages 在描述或评论保存的事务里，把文本引用到的、还在临时目录的图片搬进看板的资源子目录并入库。
// 已经入库的图片、临时目录里也找不到的编号（比如外部拼出来的）都直接跳过，不报错。
// 临时文件在事务提交后才删除，事务回滚时它还在，用户重试保存仍然能搬。
func (u *unit) commitImages(ctx context.Context, boardID int64, text string) error {
	tokens := attachmentTokens(text)
	if len(tokens) == 0 || u.core.resources == nil || u.core.uploads == nil {
		return nil
	}
	board, err := u.board.GetByID(ctx, boardID)
	if err != nil {
		return mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	for _, token := range tokens {
		if _, err := u.attachment.GetByToken(ctx, token); err == nil {
			continue
		} else if !orm.IsErrRecordNotFound(err) {
			return wrapDB("读取图片", err)
		}
		if err := u.promoteUpload(ctx, board, token); err != nil {
			return err
		}
	}
	return nil
}

// promoteUpload 把一张临时图片写进资源目录、插入记录，并登记提交后删除临时文件。临时目录里没有就什么都不做。
func (u *unit) promoteUpload(ctx context.Context, board *Board, token string) error {
	item, data, found, err := u.core.readUpload(token)
	if err != nil {
		return wrapDB("读取上传的图片", err)
	}
	if !found {
		return nil
	}
	item.BoardID = board.ID
	item.Path = AttachmentFilePath(board.Slug, token, item.ContentType)
	item.CreatedAt = u.now()
	if err := u.core.resources.Save(item.Path, data); err != nil {
		return wrapDB("保存图片文件", err)
	}
	if err := u.attachment.Create(ctx, item); err != nil {
		return wrapDB("保存图片", err)
	}
	name := token + imageExts[item.ContentType]
	u.afterCommit(func() {
		if err := u.core.uploads.Remove(name); err != nil {
			slog.Warn("清理临时上传文件失败", "name", name, "err", err)
		}
	})
	return nil
}

// releaseImages 在内容被改短、被删除后，清理文本里曾引用、现在看板里再没有人引用的图片：
// 删记录，文件在事务提交后删。别的看板的图片和仍被引用的图片都不动。
// 调用时内容的修改必须已经写入同一个事务，否则会把还在引用的图片当成无人引用。
func (u *unit) releaseImages(ctx context.Context, boardID int64, tokens []string) error {
	if u.core.resources == nil {
		return nil
	}
	for _, token := range tokens {
		item, err := u.attachment.GetByToken(ctx, token)
		if orm.IsErrRecordNotFound(err) || (err == nil && item.BoardID != boardID) {
			continue
		}
		if err != nil {
			return wrapDB("读取图片", err)
		}
		used, err := u.attachment.IsReferenced(ctx, boardID, token)
		if err != nil {
			return wrapDB("检查图片引用", err)
		}
		if used {
			continue
		}
		if err := u.attachment.DeleteByToken(ctx, token); err != nil {
			return wrapDB("删除图片记录", err)
		}
		path := item.Path
		u.afterCommit(func() {
			if err := u.core.resources.Remove(path); err != nil {
				slog.Warn("清理无人引用的图片文件失败", "path", path, "err", err)
			}
		})
	}
	return nil
}

// syncImages 描述从 oldText 改成 newText 后同步图片：新引用的入库，被删掉引用的清理。
func (u *unit) syncImages(ctx context.Context, boardID int64, oldText, newText string) error {
	if err := u.commitImages(ctx, boardID, newText); err != nil {
		return err
	}
	removed := slices.DeleteFunc(attachmentTokens(oldText), func(t string) bool {
		return slices.Contains(attachmentTokens(newText), t)
	})
	return u.releaseImages(ctx, boardID, removed)
}
