package kanban

import (
	"fmt"

	"github.com/ixugo/goddd/pkg/orm"
	"github.com/ixugo/goddd/pkg/reason"
)

// 领域错误。每个错误有独立的 reason，网页根据 reason 显示各语言的提示，
// Msg 只是没有对应翻译时的兜底文案。reason 在全局必须唯一。
var (
	// ErrInvalidActor 操作者身份不合法，说明调用方没有完成认证。
	ErrInvalidActor = reason.NewError("ErrInvalidActor", "操作者身份无效").WithHTTPStatus(401)

	// ErrInvalidInput 请求里的某个字段不合法，Msg 里说明是哪个字段、要求是什么。
	ErrInvalidInput = reason.NewError("ErrInvalidInput", "输入内容不合法")

	// ErrHumanOnly 只有人才能执行的操作，agent 调用会被拒绝。
	ErrHumanOnly = reason.NewError("ErrHumanOnly", "这个操作只允许人执行").WithHTTPStatus(403)

	// ErrBoardNotFound 看板不存在，可能已经被删除。
	ErrBoardNotFound = reason.NewError("ErrBoardNotFound", "看板不存在").WithHTTPStatus(404)

	// ErrBoardSlugTaken 看板标识已经被别的看板使用。
	ErrBoardSlugTaken = reason.NewError("ErrBoardSlugTaken", "看板标识已被使用").WithHTTPStatus(409)

	// ErrColumnNotFound 列不存在，或者不属于请求里指定的看板。
	ErrColumnNotFound = reason.NewError("ErrColumnNotFound", "列不存在").WithHTTPStatus(404)

	// ErrColumnNameTaken 同一个看板里已经有同名的列，比较时不区分大小写。
	ErrColumnNameTaken = reason.NewError("ErrColumnNameTaken", "同一个看板里已经有同名的列").WithHTTPStatus(409)

	// ErrColumnLimit 看板的列数已经到了上限。
	ErrColumnLimit = reason.NewError("ErrColumnLimit", fmt.Sprintf("每个看板最多 %d 列", maxColumnsPerBoard))

	// ErrColumnNotEmpty 要删除的列里还有卡片，又没有指定把它们移到哪一列。
	ErrColumnNotEmpty = reason.NewError("ErrColumnNotEmpty", "列里还有卡片，请指定把它们移入的列").WithHTTPStatus(409)

	// ErrColumnOrderStale 调整顺序的请求和当前的列集合对不上，说明请求依据的是过期的数据。
	ErrColumnOrderStale = reason.NewError("ErrColumnOrderStale", "列已经发生变化，请刷新后重新调整顺序").WithHTTPStatus(409)

	// ErrCardNotFound 卡片不存在，可能已经被删除。
	ErrCardNotFound = reason.NewError("ErrCardNotFound", "卡片不存在").WithHTTPStatus(404)

	// ErrCardVersionConflict 带着旧版本号提交修改，说明卡片已经被别人改过，需要刷新后再改。
	ErrCardVersionConflict = reason.NewError("ErrCardVersionConflict", "卡片已被其他人修改，请刷新后重试").WithHTTPStatus(409)

	// ErrNotHolder agent 要写的卡片当前不归它持有：没领取过、已被收回，或者被别的 agent 接手了。
	ErrNotHolder = reason.NewError("ErrNotHolder", "你当前没有持有这张卡片，请先领取").WithHTTPStatus(409)

	// ErrAgentPickupColumn agent 不能把新卡片直接放进开启了 agent 领取的列，那会绕开由人决定做什么的约定。
	ErrAgentPickupColumn = reason.NewError("ErrAgentPickupColumn", "agent 不能把卡片直接放进由 agent 处理的列").WithHTTPStatus(403)

	// ErrAgentOnly 领取、取下一张卡、拉取更新都是 agent 的概念，人在网页上直接操作，例如拖动卡片来分配工作。
	ErrAgentOnly = reason.NewError("ErrAgentOnly", "该操作只给 agent 使用，人在网页上直接操作，例如拖动卡片来分配工作").WithHTTPStatus(403)

	// ErrCardHumanOnly 卡片被标记为只能由人处理，agent 不能领取。
	ErrCardHumanOnly = reason.NewError("ErrCardHumanOnly", "这张卡片只能由人处理").WithHTTPStatus(403)

	// ErrColumnNotPickup 卡片所在的列没有开启 agent 领取。
	ErrColumnNotPickup = reason.NewError("ErrColumnNotPickup", "这一列不允许 agent 处理").WithHTTPStatus(403)

	// ErrCardWaiting 卡片依赖的卡片还没有完成，现在不能开始。
	ErrCardWaiting = reason.NewError("ErrCardWaiting", "卡片还在等待其他卡片完成").WithHTTPStatus(409)

	// ErrCardClaimed 卡片正被另一个 agent 持有，租约还没有到期。
	ErrCardClaimed = reason.NewError("ErrCardClaimed", "卡片已经被其他 agent 领取").WithHTTPStatus(409)

	// ErrRuleNotFound 自动化规则不存在，可能已经被删除。
	ErrRuleNotFound = reason.NewError("ErrRuleNotFound", "规则不存在").WithHTTPStatus(404)

	// ErrRuleLimit 看板的自动化规则数已经到了上限。
	ErrRuleLimit = reason.NewError("ErrRuleLimit", fmt.Sprintf("每个看板最多 %d 条规则", maxRulesPerBoard))

	// ErrProjectPathLocked agent 只能在项目路径为空时写入，已经有值就只有人能改。
	ErrProjectPathLocked = reason.NewError("ErrProjectPathLocked", "项目路径已经设置，只有人可以修改").WithHTTPStatus(403)
)

// invalidf 生成一个"输入不合法"的错误，Msg 是面向用户的具体说明。
func invalidf(format string, args ...any) error {
	return ErrInvalidInput.WithMsg(fmt.Sprintf(format, args...))
}

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
