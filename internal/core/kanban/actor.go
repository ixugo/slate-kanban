package kanban

import "unicode/utf8"

const (
	// maxActorNameLen 操作者名称的最大字符数。评论作者、事件操作者、领取者都用同一个上限，
	// 数据库里对应列的长度也是按它定的。
	maxActorNameLen = 64
	// automationActorPrefix 自动化规则产生的操作者名称前缀，后面接规则名，
	// 让用户在活动记录里一眼看出这是哪条规则改的。
	automationActorPrefix = "automation:"
)

// ActorKind 是操作者的类型，决定了权限和事件里的来源标记。
type ActorKind string

const (
	// ActorHuman 网页上的人，拥有看板的全部管理权限。
	ActorHuman ActorKind = "human"
	// ActorAgent 通过 MCP 或令牌接入的 AI，只能处理自己领取的任务。
	ActorAgent ActorKind = "agent"
	// ActorSystem 系统自己，例如自动化规则产生的改动。
	ActorSystem ActorKind = "system"
)

// Actor 表示一次操作是谁发起的。它由 API 层根据令牌或会话填写，
// 业务方法只信任它，不再自己判断身份，所以权限规则全部集中在 Core 里。
type Actor struct {
	Kind ActorKind
	Name string
}

// IsHuman 判断操作者是不是人。
func (a Actor) IsHuman() bool { return a.Kind == ActorHuman }

// IsAgent 判断操作者是不是 agent。agent 的权限最受限，所以代码里多数判断都写成它。
func (a Actor) IsAgent() bool { return a.Kind == ActorAgent }

// Validate 校验操作者本身是否合法：类型必须是三种之一，名称非空且不超过上限。
// 身份不合法说明调用方没有完成认证，所以返回 401 而不是 400。
func (a Actor) Validate() error {
	switch a.Kind {
	case ActorHuman, ActorAgent, ActorSystem:
	default:
		return ErrInvalidActor.Withf("未知的操作者类型 %q", a.Kind)
	}
	if n := utf8.RuneCountInString(a.Name); n == 0 || n > maxActorNameLen {
		return ErrInvalidActor.Withf("操作者名称长度必须在 1 到 %d 个字符之间", maxActorNameLen)
	}
	return nil
}

// AutomationActor 返回自动化规则执行时使用的操作者。
// 名称由固定前缀和规则名拼成，超过上限时截断规则名，保证能写进各个定长的数据库列。
func AutomationActor(ruleName string) Actor {
	room := maxActorNameLen - utf8.RuneCountInString(automationActorPrefix)
	return Actor{Kind: ActorSystem, Name: automationActorPrefix + truncateRunes(ruleName, room)}
}
