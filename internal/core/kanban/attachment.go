package kanban

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

const (
	// MaxAttachmentBytes 是单张图片的大小上限。图片存在数据库里，上限不宜太大。
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
)

// AttachmentStorer 是图片的数据持久化接口。
type AttachmentStorer interface {
	// WithTx 返回绑定到事务的副本，事务内的所有读写都必须用它。
	WithTx(orm.Tx) AttachmentStorer
	// Create 保存一张图片。
	Create(context.Context, *Attachment) error
	// GetByToken 按公开编号读取，不存在时返回可用 orm.IsErrRecordNotFound 判断的错误。
	GetByToken(context.Context, string) (*Attachment, error)
	// DeleteByBoard 删除看板的全部图片，用于删除看板。
	DeleteByBoard(context.Context, int64) error
}

// SaveAttachment 把一张图片存进看板，返回带公开编号的记录。人和 agent 都能调用；
// 类型按内容判断而不是相信上传时声明的类型，只接受 PNG、JPEG、GIF、WebP，大小不超过 5 MB。
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
	if _, err := c.store.Board().GetByID(ctx, boardID); err != nil {
		return nil, mapNotFound(err, ErrBoardNotFound, "读取看板")
	}
	raw := make([]byte, attachmentTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return nil, wrapDB("生成图片编号", err)
	}
	item := &Attachment{
		BoardID: boardID, Token: hex.EncodeToString(raw), ContentType: kind,
		Size: int64(len(data)), Data: data, CreatedAt: c.now(),
	}
	if err := c.store.Attachment().Create(ctx, item); err != nil {
		return nil, wrapDB("保存图片", err)
	}
	return item, nil
}

// GetAttachment 按公开编号读取图片。编号格式不对直接当作不存在，不去查库。
func (c Core) GetAttachment(ctx context.Context, token string) (*Attachment, error) {
	if !attachmentTokenRe.MatchString(token) {
		return nil, ErrAttachmentNotFound
	}
	item, err := c.store.Attachment().GetByToken(ctx, token)
	if err != nil {
		return nil, mapNotFound(err, ErrAttachmentNotFound, "读取图片")
	}
	return item, nil
}
