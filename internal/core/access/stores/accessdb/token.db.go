package accessdb

import (
	"context"
	"time"

	"slate-kanban/internal/core/access"
)

var _ access.TokenStorer = Token{}

// Token 是令牌的数据库实现，和 DB 同一个底层结构，通过类型转换得到。
type Token DB

// Create 新增一个令牌。
func (d Token) Create(ctx context.Context, model *access.Token) error {
	return d.db.WithContext(ctx).Create(model).Error
}

// GetByHash 按令牌哈希读取。
func (d Token) GetByHash(ctx context.Context, hash string) (*access.Token, error) {
	var out access.Token
	if err := d.db.WithContext(ctx).Where("hash = ?", hash).Take(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

// List 返回全部令牌，按主键升序，也就是创建顺序。
func (d Token) List(ctx context.Context) ([]*access.Token, error) {
	items := make([]*access.Token, 0)
	if err := d.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Delete 按主键删除，幂等。
func (d Token) Delete(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Where("id = ?", id).Delete(&access.Token{}).Error
}

// Touch 只更新最后使用时间，不碰其他字段。
func (d Token) Touch(ctx context.Context, id int64, at time.Time) error {
	return d.db.WithContext(ctx).Model(&access.Token{}).Where("id = ?", id).Update("last_used_at", at).Error
}
