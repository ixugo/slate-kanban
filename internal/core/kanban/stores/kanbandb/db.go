// Package kanbandb 是看板领域的数据库实现（被动适配器）。
package kanbandb

import (
	"github.com/ixugo/goddd/pkg/orm"
	"gorm.io/gorm"

	"slate-kanban/internal/core/kanban"
)

var _ kanban.Storer = DB{}

// DB 是看板领域各实体存储的统一入口。它只有一个指针字段，
// 各实体存储通过类型转换从它得到（例如 Board(d)），不会产生额外的内存分配。
type DB struct {
	db *gorm.DB
}

// NewDB 用 GORM 连接创建看板领域的数据库存储。
func NewDB(db *gorm.DB) DB {
	return DB{db: db}
}

// Begin 开启一个事务，返回 orm.Tx 句柄。
func (d DB) Begin() (orm.Tx, error) {
	return orm.Begin(d.db)
}

// Board 返回看板存储。
func (d DB) Board() kanban.BoardStorer {
	return Board(d)
}

// Column 返回看板列存储。
func (d DB) Column() kanban.ColumnStorer {
	return Column(d)
}

// Card 返回卡片存储。
func (d DB) Card() kanban.CardStorer {
	return Card(d)
}

// CardChange 返回卡片变更日志存储。
func (d DB) CardChange() kanban.CardChangeStorer {
	return CardChange(d)
}

// Comment 返回卡片评论存储。
func (d DB) Comment() kanban.CommentStorer {
	return Comment(d)
}

// Attachment 返回图片存储。
func (d DB) Attachment() kanban.AttachmentStorer {
	return Attachment(d)
}

// Rule 返回自动化规则存储。
func (d DB) Rule() kanban.RuleStorer {
	return Rule(d)
}

// Tag 返回标签存储。
func (d DB) Tag() kanban.TagStorer {
	return Tag(d)
}

// Engineer 返回工程师名册存储。
func (d DB) Engineer() kanban.EngineerStorer {
	return Engineer(d)
}

// ProjectTemplate 返回自定义项目模板存储。
func (d DB) ProjectTemplate() kanban.ProjectTemplateStorer {
	return ProjectTemplate(d)
}

// Event 返回事件日志存储。
func (d DB) Event() kanban.EventStorer {
	return Event(d)
}

// Cursor 返回 agent 读取进度存储。
func (d DB) Cursor() kanban.CursorStorer {
	return Cursor(d)
}

// AutoMigrate 同步表结构：已有的表只会补齐缺失的列和索引，不会删除数据，每次启动都可以重复执行。
// 项目还在开发阶段，没有为旧版本数据写迁移逻辑：表结构变了就由开发者用独立程序处理本地数据库。
func (d DB) AutoMigrate(ok bool) DB {
	if !ok {
		return d
	}
	if err := d.db.AutoMigrate(
		new(kanban.Board), new(kanban.Column), new(kanban.Card), new(kanban.CardChange), new(kanban.Comment),
		new(kanban.Rule), new(kanban.Event), new(kanban.AgentCursor), new(kanban.Attachment), new(kanban.Tag), new(kanban.Engineer),
		new(kanban.ProjectTemplate),
	); err != nil {
		panic(err)
	}
	return d
}
