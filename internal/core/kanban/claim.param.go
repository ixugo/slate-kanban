package kanban

// ClaimCardInput 是领取卡片的入参。LeaseMinutes 是租约时长（分钟），0 表示用默认值。
type ClaimCardInput struct {
	LeaseMinutes int `json:"lease_minutes"`
}

// NextCardInput 是取下一张卡的入参，租约时长的含义同 ClaimCardInput。
type NextCardInput struct {
	LeaseMinutes int `json:"lease_minutes"`
}

// FinishCardInput 是完成交接的入参。Summary 是必填的工作总结，会作为评论写在卡片下；
// Branch 和 URL 为空表示不改；ColumnID 为 0 表示移到当前列设置的下一列。
type FinishCardInput struct {
	Summary  string `json:"summary"`
	Branch   string `json:"branch"`
	URL      string `json:"url"`
	ColumnID int64  `json:"column_id"`
}
