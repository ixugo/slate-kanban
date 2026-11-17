package kanban

import (
	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

// 领域错误。每个错误有独立的 reason，网页根据 reason 显示各语言的提示，
// Msg 只是没有对应翻译时的兜底文案。reason 在全局必须唯一。
var (
	// ErrInvalidActor 操作者身份不合法，说明调用方没有完成认证。
	ErrInvalidActor = reason.NewError("ErrInvalidActor", "操作者身份无效").WithHTTPStatus(401)

	// ErrBoardNotFound 看板不存在，可能已经被删除。
	ErrBoardNotFound = reason.NewError("ErrBoardNotFound", "看板不存在").WithHTTPStatus(404)
)

// wrapDB 把存储层返回的错误包装成统一的数据库错误：对外只暴露通用提示，
// 具体原因放进 details 并保留错误链，方便排查又不泄露内部细节。op 说明当时在做什么。
func wrapDB(op string, err error) error {
	return reason.ErrDB.WithCause(err).Withf("%s: %v", op, err)
}

// mapNotFound 把存储层的"找不到记录"转成指定的领域错误，其他错误按数据库错误处理。
func mapNotFound(err error, notFound error, op string) error {
	if orm.IsErrRecordNotFound(err) {
		return notFound
	}
	return wrapDB(op, err)
}
