package kanban

import "time"

// Card 卡片，是看板里被人和 agent 共同处理的工作项。
//
// 几个容易混淆的字段：
//   - Key 由看板 slug 和 Number 组成（例如 SLATE-12），对人和 agent 都是稳定的引用名。
//   - Position 是列内的排序位置，用浮点数，插到两张卡之间时取中点，不用整列重写。
//   - ClaimedBy 与 ClaimExpiresAt 表示哪个 agent 在做这张卡、占用到什么时候；
//     ClaimEngineerID 是这次领取由名册里的哪位工程师承担，EngineerID 是卡片绑定（或首次领取时随机分配）的研发工程师。
//   - Version 是乐观锁，每次真实修改加一。
//
// 时间字段用指针，nil 表示还没有发生，序列化成 JSON 的 null。
type Card struct {
	ID              int64      `gorm:"primaryKey" json:"id"`
	BoardID         int64      `gorm:"column:board_id;notNull;default:0;index:idx_cards_board_column_pos,priority:1;comment:所属看板" json:"board_id"`
	ColumnID        int64      `gorm:"column:column_id;notNull;default:0;index:idx_cards_board_column_pos,priority:2;comment:所在列" json:"column_id"`
	Number          int64      `gorm:"column:number;notNull;default:0;comment:看板内的卡片编号" json:"number"`
	Key             string     `gorm:"column:key;size:32;notNull;default:'';uniqueIndex:uq_cards_key;comment:卡片编号，由看板 slug 和编号组成" json:"key"`
	Title           string     `gorm:"column:title;size:200;notNull;default:'';comment:标题" json:"title"`
	Description     string     `gorm:"column:description;type:text;notNull;default:'';comment:描述" json:"description"`
	Position        float64    `gorm:"column:position;notNull;default:0;index:idx_cards_board_column_pos,priority:3;comment:列内排序位置，越小越靠上" json:"position"`
	Priority        int        `gorm:"column:priority;notNull;default:0;comment:优先级，0 未设置，1 低，2 中，3 高" json:"priority"`
	TagIDs          []int64    `gorm:"column:tag_ids;type:text;serializer:json;notNull;default:'[]';comment:标签主键，JSON 数组，指向 tags 表" json:"tag_ids"`
	HumanOnly       bool       `gorm:"column:human_only;notNull;default:FALSE;comment:是否只能由人处理" json:"human_only"`
	WaitsOn         []int64    `gorm:"column:waits_on;type:text;serializer:json;notNull;default:'[]';comment:依赖的卡片主键，JSON 数组" json:"waits_on"`
	DueAt           *time.Time `gorm:"column:due_at;comment:到期时间" json:"due_at"`
	StartedAt       *time.Time `gorm:"column:started_at;comment:开始时间" json:"started_at"`
	FinishedAt      *time.Time `gorm:"column:finished_at;comment:完成时间" json:"finished_at"`
	ClaimedBy       string     `gorm:"column:claimed_by;size:64;notNull;default:'';comment:当前持有这张卡的 agent 名称" json:"claimed_by"`
	ClaimExpiresAt  *time.Time `gorm:"column:claim_expires_at;comment:领取的到期时间" json:"claim_expires_at"`
	EngineerID      int64      `gorm:"column:engineer_id;notNull;default:0;comment:绑定或分配的研发工程师，指向 board_engineers 表，0 表示还没有" json:"engineer_id"`
	ClaimEngineerID int64      `gorm:"column:claim_engineer_id;notNull;default:0;comment:当前领取这张卡的工程师，领取放开后清零" json:"claim_engineer_id"`
	Branch          string     `gorm:"column:branch;size:200;notNull;default:'';comment:关联的代码分支" json:"branch"`
	URL             string     `gorm:"column:url;size:500;notNull;default:'';comment:关联的链接，例如合并请求" json:"url"`
	Version         int64      `gorm:"column:version;notNull;default:1;comment:乐观锁版本号，每次真实修改加一" json:"version"`
	CreatedBy       string     `gorm:"column:created_by;size:64;notNull;default:'';comment:创建者名称" json:"created_by"`
	UpdatedBy       string     `gorm:"column:updated_by;size:64;notNull;default:'';comment:最后修改者名称" json:"updated_by"`
	CreatedAt       time.Time  `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Card) TableName() string {
	return "cards"
}

// Normalize 把空的标签和依赖统一成空切片，这样 JSON 里永远是 [] 而不是 null，
// 前端和 agent 不用到处判空。存储层每次读出卡片后都要调用它。
func (c *Card) Normalize() {
	if c.TagIDs == nil {
		c.TagIDs = []int64{}
	}
	if c.WaitsOn == nil {
		c.WaitsOn = []int64{}
	}
}
