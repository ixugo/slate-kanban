package kanban

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"regexp"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

const (
	// MaxAttachmentBytes 是单张图片的大小上限。图片整张读进内存处理，上限不宜太大。
	MaxAttachmentBytes = 5 << 20
	// attachmentTokenBytes 是随机编号的字节数，十六进制后是 32 个字符。
	attachmentTokenBytes = 16
	// sniffLen 是识别图片类型时最多看的字节数，和 http.DetectContentType 的上限一致。
	sniffLen = 512
)

// attachmentTokenRe 是合法的图片编号：32 个小写十六进制字符。
var attachmentTokenRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// allowedImageTypes 是允许保存的图片类型。故意不含 SVG：SVG 能带脚本，直接访问时会被浏览器执行。
var allowedImageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

var (
	// ErrAttachmentTooLarge 图片超过大小上限。
	ErrAttachmentTooLarge = reason.NewError("ErrAttachmentTooLarge", "图片太大，最大 5 MB").WithHTTPStatus(413)

	// ErrAttachmentNotFound 图片不存在，可能所在的看板已经被删除。
	ErrAttachmentNotFound = reason.NewError("ErrAttachmentNotFound", "图片不存在").WithHTTPStatus(404)

	// ErrResourcesDisabled 没有配置资源目录，不能保存和读取图片。
	ErrResourcesDisabled = reason.NewError("ErrResourcesDisabled", "未配置资源目录，无法处理图片").WithHTTPStatus(500)
)

// imageExts 是图片类型对应的文件扩展名。
var imageExts = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}

// ResourceStorer 是图片等资源文件的存放接口。路径都是相对资源目录的，
// 实现必须拒绝绝对路径和带 .. 的路径，不能写到资源目录之外。
type ResourceStorer interface {
	// Save 保存文件，同名文件会被覆盖；上级目录不存在时自动创建。
	Save(rel string, data []byte) error
	// Read 读取文件，不存在时返回 os.ErrNotExist 能识别的错误。
	Read(rel string) ([]byte, error)
	// Remove 删除文件，文件不存在也算成功。
	Remove(rel string) error
	// RemoveDir 删除一个子目录和里面的全部文件，目录不存在也算成功。
	RemoveDir(dir string) error
}

// AttachmentFilePath 返回图片文件相对资源目录的路径：看板 slug 是子目录，文件名是编号加扩展名。
// 类型不在白名单里时扩展名为空，这种文件不会由正常上传产生。
func AttachmentFilePath(slug, token, contentType string) string {
	return slug + "/" + token + imageExts[contentType]
}

// AttachmentStorer 是图片的数据持久化接口。
type AttachmentStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) AttachmentStorer
	// Create 保存一张图片的记录（不含文件内容）。
	Create(context.Context, *Attachment) error
	// GetByToken 按公开编号读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByToken(context.Context, string) (*Attachment, error)
	// DeleteByBoard 删除看板的全部图片，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// SaveAttachment 把一张图片存进看板，返回带公开编号的记录。人和 agent 都能调用；
// 类型按内容判断而不是相信上传时声明的类型，只接受 PNG、JPEG、GIF、WebP，大小不超过 5 MB。
// 先写文件再写数据库记录，记录写失败时把刚写的文件删掉，不留孤儿文件。
func (c Core) SaveAttachment(ctx context.Context, actor Actor, boardID int64, data []byte) (*Attachment, error) {
	if err := requireWriter(actor); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, invalidf("图片内容不能为空")
	}
	if len(data) > MaxAttachmentBytes {
		return nil, ErrAttachmentTooLarge
	}
	kind := http.DetectContentType(data[:min(len(data), sniffLen)])
	if !allowedImageTypes[kind] {
		return nil, invalidf("只支持 PNG、JPEG、GIF、WebP 图片")
	}
	if c.resources == nil {
		return nil, ErrResourcesDisabled
	}
	board, err := c.store.Board().GetByID(ctx, boardID)
	if err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	raw := make([]byte, attachmentTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, wrapDB("生成图片编号", err)
	}
	token := hex.EncodeToString(raw)
	item := &Attachment{
		BoardID: boardID, Token: token, ContentType: kind, Size: int64(len(data)),
		Path: AttachmentFilePath(board.Slug, token, kind), CreatedAt: c.now(),
	}
	if err := c.resources.Save(item.Path, data); err != nil {
		return nil, wrapDB("保存图片文件", err)
	}
	if err := c.store.Attachment().Create(ctx, item); err != nil {
		if rmErr := c.resources.Remove(item.Path); rmErr != nil {
			slog.Warn("清理未入库的图片文件失败", "path", item.Path, "err", rmErr)
		}
		return nil, wrapDB("保存图片", err)
	}
	return item, nil
}

// GetAttachment 按公开编号读取图片记录和文件内容。编号格式不对直接当作不存在，不去查库；
// 记录在但文件丢了（比如资源目录被手动清理）也按不存在处理，并记一条警告方便排查。
func (c Core) GetAttachment(ctx context.Context, token string) (*Attachment, []byte, error) {
	if !attachmentTokenRe.MatchString(token) {
		return nil, nil, ErrAttachmentNotFound
	}
	if c.resources == nil {
		return nil, nil, ErrResourcesDisabled
	}
	item, err := c.store.Attachment().GetByToken(ctx, token)
	if err != nil {
		return nil, nil, mapNotFound(err, ErrAttachmentNotFound, "读取图片")
	}
	data, err := c.resources.Read(item.Path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Warn("图片记录存在但文件丢失", "path", item.Path)
		return nil, nil, ErrAttachmentNotFound
	}
	if err != nil {
		return nil, nil, wrapDB("读取图片文件", err)
	}
	return item, data, nil
}
