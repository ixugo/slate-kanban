package kanban

import "time"

const (
	// WebhookTypeDingTalk 钉钉群机器人。
	WebhookTypeDingTalk = "dingtalk"
	// WebhookTypeWeCom 企业微信群机器人。
	WebhookTypeWeCom = "wecom"
	// WebhookTypeFeishu 飞书/Lark 群机器人。
	WebhookTypeFeishu = "feishu"
	// MaxWebhooksPerBoard 单看板最多几条 webhook，避免活动密集时对外部服务造成压力。
	MaxWebhooksPerBoard = 20
	// MaxWebhookNameLen webhook 名称最大字符数。
	MaxWebhookNameLen = 100
	// MaxWebhookURLLen webhook 地址最大字符数，前后端与测试共用此导出名。
	MaxWebhookURLLen = 512
	// MaxWebhookSecretLen 签名密钥最大字符数。
	MaxWebhookSecretLen = 200
)

// Webhook 是看板的一条活动通知 webhook 配置，密钥明文存库但不对外返回。
type Webhook struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	BoardID       int64     `gorm:"column:board_id;notNull;default:0;index:idx_webhooks_board;comment:所属看板" json:"board_id"`
	Name          string    `gorm:"column:name;size:100;notNull;default:'';comment:显示名称" json:"name"`
	Type          string    `gorm:"column:type;size:16;notNull;default:'';comment:dingtalk/wecom/feishu" json:"type"`
	URL           string    `gorm:"column:url;size:512;notNull;default:'';comment:HTTPS 群机器人地址" json:"url"`
	Secret        string    `gorm:"column:secret;size:200;notNull;default:'';comment:可选签名密钥" json:"-"`
	Enabled       bool      `gorm:"column:enabled;notNull;comment:是否启用" json:"enabled"`
	EventTypes    []string  `gorm:"column:event_types;type:text;serializer:json;notNull;default:'[]';comment:订阅事件，空表示全部" json:"event_types"`
	ActorKinds    []string  `gorm:"column:actor_kinds;type:text;serializer:json;notNull;default:'[]';comment:订阅操作者类型，空表示全部" json:"actor_kinds"`
	MoveColumnIDs []int64   `gorm:"column:move_column_ids;type:text;serializer:json;notNull;default:'[]';comment:card.moved 目标列，空表示全部" json:"move_column_ids"`
	CreatedAt     time.Time `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// TableName 指定表名。
func (*Webhook) TableName() string { return "webhooks" }

// Normalize 把空切片统一成非 nil，便于 JSON 序列化。
func (w *Webhook) Normalize() {
	if w.EventTypes == nil {
		w.EventTypes = []string{}
	}
	if w.ActorKinds == nil {
		w.ActorKinds = []string{}
	}
	if w.MoveColumnIDs == nil {
		w.MoveColumnIDs = []int64{}
	}
}

// WebhookView 是对外返回的 webhook，不含明文密钥。
type WebhookView struct {
	ID            int64     `json:"id"`
	BoardID       int64     `json:"board_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	URL           string    `json:"url"`
	SecretSet     bool      `json:"secret_set"`
	SecretMask    string    `json:"secret_mask"`
	Enabled       bool      `json:"enabled"`
	EventTypes    []string  `json:"event_types"`
	ActorKinds    []string  `json:"actor_kinds"`
	MoveColumnIDs []int64   `json:"move_column_ids"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// webhookView 把库里的 webhook 转成 API 视图并脱敏密钥。
func webhookView(w *Webhook) WebhookView {
	w.Normalize()
	mask := ""
	set := w.Secret != ""
	if set {
		mask = "******"
	}
	return WebhookView{
		ID: w.ID, BoardID: w.BoardID, Name: w.Name, Type: w.Type, URL: w.URL,
		SecretSet: set, SecretMask: mask, Enabled: w.Enabled,
		EventTypes: w.EventTypes, ActorKinds: w.ActorKinds, MoveColumnIDs: w.MoveColumnIDs,
		CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt,
	}
}
