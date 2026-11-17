package access

import "time"

// Token 是一个 agent 的访问令牌。库里只存令牌的哈希，拿不到明文：
// 明文只在创建的那一次返回给人，之后无法找回，丢了只能删掉重建。
type Token struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"column:name;size:64;notNull;default:'';uniqueIndex:uq_agent_tokens_name;comment:令牌绑定的 agent 名称，也是它在看板里的操作者名称" json:"name"`
	Hash       string     `gorm:"column:hash;size:64;notNull;default:'';uniqueIndex:uq_agent_tokens_hash;comment:令牌的 SHA-256 十六进制串" json:"-"`
	Prefix     string     `gorm:"column:prefix;size:16;notNull;default:'';comment:令牌的前几个字符，列表里用来辨认是哪一个" json:"prefix"`
	LastUsedAt *time.Time `gorm:"column:last_used_at;comment:最后一次认证成功的时间，一分钟内不重复记录" json:"last_used_at"`
	CreatedAt  time.Time  `gorm:"column:created_at;notNull;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
}

// TableName 指定表名，这是 GORM 约定的方法。
func (*Token) TableName() string {
	return "agent_tokens"
}
