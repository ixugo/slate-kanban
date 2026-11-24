package kanban

// ClaimCardInput 是领取任务的入参。LeaseMinutes 是租约时长（分钟），0 表示用默认值。
type ClaimCardInput struct {
	LeaseMinutes int `json:"lease_minutes"`
	// EngineerID 是由哪位工程师承担这次领取，0 表示由服务端挑：任务绑定了就用绑定的，否则随机挑一位空闲的。
	EngineerID int64 `json:"engineer_id"`
	// DesktopDispatch 由桌面调度设置，只领取空闲任务，并在事件中保存工程师名称。
	DesktopDispatch bool `json:"-" form:"-"`
	// RenewOnly 由桌面心跳设置，只续当前工程师的有效租约。
	RenewOnly bool `json:"-" form:"-"`
}

// NextCardInput 是取下一张卡的入参，租约时长的含义同 ClaimCardInput。
type NextCardInput struct {
	ColumnID     int64 `json:"column_id"` // 必填；agent 阅读列提示词后选择处理的列。
	LeaseMinutes int   `json:"lease_minutes"`
	EngineerID   int64 `json:"engineer_id"`
}

// FinishCardInput 是完成交接的入参。Summary 是必填的工作总结，会作为评论写在任务下；
// Branch 和 URL 为空表示不改；ColumnID 为 0 表示移到当前列设置的下一列。
type FinishCardInput struct {
	Summary  string `json:"summary"`
	Branch   string `json:"branch"`
	URL      string `json:"url"`
	ColumnID int64  `json:"column_id"`
	// BeforeID、AfterID 指定任务在目标列里放到哪项任务的前面或后面，最多填一个。
	BeforeID int64 `json:"before_id"`
	AfterID  int64 `json:"after_id"`
	// Top 为真且没有指定 BeforeID、AfterID 时放到目标列最顶部，否则放到列尾；只由 MCP 设置，不出现在 REST 的请求体里。
	Top bool `json:"-"`
	// MergedCommits 和 Version 由 MCP 审查交接提供，REST 仍使用原有交接协议。
	MergedCommits []string `json:"-"`
	Version       *int64   `json:"-"`
}
