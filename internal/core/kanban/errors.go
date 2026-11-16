package kanban

import "github.com/ixugo/goddd/pkg/reason"

// 领域错误。每个错误有独立的 reason，网页根据 reason 显示各语言的提示，
// Msg 只是没有对应翻译时的兜底文案。reason 在全局必须唯一。
var (
	// ErrInvalidActor 操作者身份不合法，说明调用方没有完成认证。
	ErrInvalidActor = reason.NewError("ErrInvalidActor", "操作者身份无效").WithHTTPStatus(401)
)
