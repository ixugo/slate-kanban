package kanbandb

import (
	"context"

	"github.com/ixugo/goddd/pkg/orm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.AttachmentStorer = Attachment{}

// Attachment 是图片的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Attachment DB

// WithTx 返回绑定到事务的副本。
func (d Attachment) WithTx(tx orm.Tx) kanban.AttachmentStorer {
	return Attachment{db: orm.GormDB(tx)}
}

// Create 保存一张图片。
func (d Attachment) Create(ctx context.Context, model *kanban.Attachment) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// GetByToken 按公开编号读取，不存在时返回 gorm 的 record not found。
func (d Attachment) GetByToken(ctx context.Context, token string) (*kanban.Attachment, error) {
	var out kanban.Attachment
	if err := d.db.WithContext(ctx).Where("token = ?", token).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteByBoard 删除看板的全部图片。
func (d Attachment) DeleteByBoard(ctx context.Context, boardID int64) error {
	return d.db.WithContext(ctx).Where("board_id = ?", boardID).Delete(&kanban.Attachment{}).Error
}

// DeleteByToken 删除一张图片的记录，记录不存在也算成功。
func (d Attachment) DeleteByToken(ctx context.Context, token string) error {
	return d.db.WithContext(ctx).Where("token = ?", token).Delete(&kanban.Attachment{}).Error
}

// IsReferenced 看板里是否还有卡片描述或评论正文包含这个编号。编号由调用方保证是 32 位十六进制，
// 不含 LIKE 的通配符。每张表一次带 LIMIT 1 的查询，没有命中才会查下一张。
func (d Attachment) IsReferenced(ctx context.Context, boardID int64, token string) (bool, error) {
	for _, src := range []struct{ table, column string }{{"cards", "description"}, {"comments", "body"}} {
		var n int64
		err := d.db.WithContext(ctx).Table(src.table).Where("board_id = ? AND "+src.column+" LIKE ?", boardID, "%"+token+"%").Limit(1).Count(&n).Error
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}
