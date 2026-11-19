package kanban

import "time"

// 工程师的类型：全栈、前端、后端、UI 都负责研发，测试负责验收。看板里每个工程师就是一个并发名额：
// 有几个研发类工程师就最多同时做几张研发卡，有几个测试就最多同时审几张卡。
// 研发类的几种只是专长不同，领取规则完全一样，调度 agent 可以按专长选人。
const (
	// EngineerDev 全栈工程师，领取「进行中」这类 agent 取卡列的卡片。
	EngineerDev = "dev"
	// EngineerFrontend 前端工程师，和全栈一样做研发。
	EngineerFrontend = "frontend"
	// EngineerBackend 后端工程师，和全栈一样做研发。
	EngineerBackend = "backend"
	// EngineerUI UI 工程师，和全栈一样做研发。
	EngineerUI = "ui"
	// EngineerReview 测试工程师，领取其他列（通常是「审查」）的卡片。
	EngineerReview = "review"
)

// stageOf 返回一种工程师类型干哪个阶段的活：测试干审查阶段（EngineerReview），其余都干研发阶段（EngineerDev）。
func stageOf(kind string) string {
	if kind == EngineerReview {
		return EngineerReview
	}
	return EngineerDev
}

// Engineer 是看板名册里的一位工程师：一个名字、一个类型和一段角色提示词。
// 调度 agent 读到名册后，按名额数量派子 agent，并把角色提示词连同任务原文交给子 agent。
// 工程师属于一个看板，看板删除时一并删除。
type Engineer struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	BoardID   int64     `gorm:"column:board_id;notNull;default:0;index:idx_engineers_board_sort,priority:1;comment:所属看板" json:"-"`
	Name      string    `gorm:"column:name;size:30;notNull;default:'';comment:工程师名称，同一个看板里不重名" json:"name"`
	Kind      string    `gorm:"column:kind;size:16;notNull;default:'dev';comment:类型，dev 全栈，frontend 前端，backend 后端，ui UI，review 测试" json:"kind"`
	Prompt    string    `gorm:"column:prompt;type:text;notNull;default:'';comment:角色提示词，调度 agent 派子 agent 时使用" json:"prompt"`
	Avatar    string    `gorm:"column:avatar;size:32;notNull;default:'';comment:头像的随机种子，网页按它画头像；空表示用工程师编号" json:"avatar"`
	Sort      int64     `gorm:"column:sort;notNull;default:0;index:idx_engineers_board_sort,priority:2;comment:在名册里的顺序，越小越靠前" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"-"`
	UpdatedAt time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"-"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Engineer) TableName() string {
	return "board_engineers"
}

// EngineerInput 是修改名册时的一项入参。ID 为 0 表示招募，由服务端分配；
// 非 0 必须是名册里已有的主键，内容不同于原来就是修改。入参的顺序就是名册的显示顺序。
type EngineerInput struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Prompt string `json:"prompt"`
	// Avatar 是头像的随机种子：招募时留空由服务端随机生成，修改时留空表示沿用原来的。
	Avatar string `json:"avatar"`
}
