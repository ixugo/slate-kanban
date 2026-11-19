package kanbandb

import (
	"fmt"
	"log/slog"

	"slate-kanban/internal/core/kanban"
)

// legacyAttachment 是旧版把图片字节存在数据库时，搬迁要读的一行（连带所属看板的 slug）。
type legacyAttachment struct {
	Token       string
	ContentType string
	Data        []byte
	Slug        string
}

// MigrateAttachmentFiles 把旧版存在 attachments.data 里的图片搬成资源目录下的文件，
// 数据库只留相对路径，全部搬完后删掉 data 列。启动时在表结构同步之后调用，可以重复执行：
//   - 没有 data 列说明已经搬完或是全新的库，直接返回；
//   - 一张一张处理：先写文件、再记路径，中途崩溃重启后会接着搬 path 还是空的那些；
//   - 所属看板已不存在的孤儿图片没人能引用，直接删除记录。
//
// 图片数量预期在几百张以内，每张一次读一次写，总体 O(n)。
func (d DB) MigrateAttachmentFiles(res kanban.ResourceStorer) error {
	if !d.db.Migrator().HasColumn("attachments", "data") {
		return nil
	}
	var ids []int64
	if err := d.db.Raw("SELECT id FROM attachments WHERE path = ''").Scan(&ids).Error; err != nil {
		return fmt.Errorf("读取待搬迁图片: %w", err)
	}
	for _, id := range ids {
		if err := d.moveAttachmentToFile(res, id); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		slog.Info("图片已从数据库搬到资源目录", "count", len(ids))
	}
	if err := d.db.Exec("ALTER TABLE attachments DROP COLUMN data").Error; err != nil {
		return fmt.Errorf("删除 attachments.data 列: %w", err)
	}
	return nil
}

// moveAttachmentToFile 搬迁一张图片：写文件成功后才更新路径，写失败时记录保持原样，下次启动重试。
func (d DB) moveAttachmentToFile(res kanban.ResourceStorer, id int64) error {
	var row legacyAttachment
	err := d.db.Raw("SELECT a.token, a.content_type, a.data, b.slug FROM attachments a JOIN boards b ON b.id = a.board_id WHERE a.id = ?", id).Scan(&row).Error
	if err != nil {
		return fmt.Errorf("读取图片 %d: %w", id, err)
	}
	if row.Token == "" {
		return d.db.Exec("DELETE FROM attachments WHERE id = ?", id).Error
	}
	path := kanban.AttachmentFilePath(row.Slug, row.Token, row.ContentType)
	if err := res.Save(path, row.Data); err != nil {
		return fmt.Errorf("写入图片文件 %s: %w", path, err)
	}
	return d.db.Exec("UPDATE attachments SET path = ? WHERE id = ?", path, id).Error
}
