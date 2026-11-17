package kanban

// ClaimCardInput 是领取卡片的入参。LeaseMinutes 是租约时长（分钟），0 表示用默认值。
type ClaimCardInput struct {
	LeaseMinutes int `json:"lease_minutes"`
}

// NextCardInput 是取下一张卡的入参，租约时长的含义同 ClaimCardInput。
type NextCardInput struct {
	LeaseMinutes int `json:"lease_minutes"`
}
