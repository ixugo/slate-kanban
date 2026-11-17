package access

import "github.com/ixugo/goddd/pkg/reason"

var (
	// ErrUnauthorized 令牌缺失、格式不对、已被删除或者根本不存在，调用方需要重新提供有效的令牌。
	ErrUnauthorized = reason.ErrUnauthorized.WithMsg("令牌无效或已被删除")

	// ErrAdminOnly 令牌管理只允许管理令牌，agent 不能给自己发令牌，也不能删别人的。
	ErrAdminOnly = reason.NewError("ErrAdminOnly", "只有管理令牌可以管理 agent 令牌").WithHTTPStatus(403)

	// ErrTokenNameTaken 已经有一个令牌用了这个名称，名称就是 agent 在看板里的操作者名称，必须唯一。
	ErrTokenNameTaken = reason.NewError("ErrTokenNameTaken", "这个名称已经有令牌在用了").WithHTTPStatus(409)

	// ErrInvalidName 令牌名称不符合规则。
	ErrInvalidName = reason.NewError("ErrInvalidName", "令牌名称不合法")
)

// wrapDB 把存储层返回的错误包装成统一的数据库错误：对外只暴露通用提示，
// 具体原因放进 details 并保留错误链。op 说明当时在做什么。
func wrapDB(op string, err error) error {
	return reason.ErrDB.WithCause(err).Withf("%s: %v", op, err)
}
