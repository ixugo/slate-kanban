package kanban

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DescribeEventZH 用中文白话描述一条活动事件，供 webhook 推送正文使用。
func DescribeEventZH(ev *Event) string {
	var p map[string]any
	if ev.Payload != "" && ev.Payload != "{}" {
		_ = json.Unmarshal([]byte(ev.Payload), &p)
	}
	field := func(key string) string {
		if p == nil {
			return ""
		}
		v, ok := p[key]
		if !ok {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return fmt.Sprint(v)
		}
		return s
	}
	fields := func(key string) []string {
		if p == nil {
			return nil
		}
		v, ok := p[key]
		if !ok {
			return nil
		}
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		out := make([]string, 0, len(arr))
		for _, it := range arr {
			if s, ok := it.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	plain := func(s string) string { return s }
	switch ev.Type {
	case EventBoardCreated:
		return plain("创建了看板")
	case EventBoardUpdated:
		return plain("修改了看板信息")
	case EventBoardLabelsUpdated:
		return plain("更新了看板标签库")
	case EventColumnCreated:
		return plain(fmt.Sprintf("新增列「%s」", field("name")))
	case EventColumnUpdated:
		return plain(fmt.Sprintf("修改列「%s」", field("name")))
	case EventColumnDeleted:
		return plain(fmt.Sprintf("删除列「%s」", field("name")))
	case EventColumnReordered:
		return plain("调整了列顺序")
	case EventCardCreated:
		return plain(fmt.Sprintf("在「%s」创建了任务", field("column")))
	case EventCardUpdated:
		labels := fields("fields")
		if len(labels) == 0 {
			return plain("修改了任务")
		}
		return plain("修改了任务的 " + strings.Join(labels, "、"))
	case EventCardTagsUpdated:
		return plain("更新了任务标签")
	case EventCardMoved:
		return plain(fmt.Sprintf("将任务从「%s」移到「%s」", field("from"), field("to")))
	case EventCardReordered:
		return plain("调整了任务顺序")
	case EventCardDeleted:
		return plain("删除了任务")
	case EventCardClaimed:
		return plain("领取了任务")
	case EventCardReleased:
		switch field("why") {
		case "revoked":
			return plain(fmt.Sprintf("收回了 agent「%s」对任务的持有", field("agent")))
		case "moved":
			return plain(fmt.Sprintf("任务换列，结束了 agent「%s」的持有", field("agent")))
		case releaseExpired:
			return plain(fmt.Sprintf("发现 agent「%s」的租约已到期，领取结束", field("agent")))
		default:
			return plain("释放了任务")
		}
	case EventCommentAdded:
		snippet := field("snippet")
		if snippet == "" {
			return plain("添加了评论")
		}
		return plain("添加了评论：" + snippet)
	case EventCommentDeleted:
		return plain("删除了评论")
	case EventRuleCreated:
		return plain(fmt.Sprintf("新增自动化规则「%s」", field("name")))
	case EventRuleUpdated:
		return plain(fmt.Sprintf("修改自动化规则「%s」", field("name")))
	case EventRuleDeleted:
		return plain(fmt.Sprintf("删除自动化规则「%s」", field("name")))
	case EventAutomationFired:
		acts := fields("actions")
		if len(acts) == 0 {
			return plain("自动化规则已执行")
		}
		return plain("自动化规则已执行：" + strings.Join(acts, "、"))
	default:
		return plain(fmt.Sprintf("发生了事件 %s", ev.Type))
	}
}

// FormatWebhookMessage 拼出 webhook 正文，包含看板名、任务编号标题与事件描述。
func FormatWebhookMessage(boardName string, ev *Event) string {
	desc := DescribeEventZH(ev)
	var b strings.Builder
	b.WriteString("【")
	b.WriteString(boardName)
	b.WriteString("】")
	if ev.CardID != 0 {
		key, title := eventCardKeyTitle(ev.Payload)
		if key != "" {
			b.WriteString(key)
			if title != "" {
				b.WriteString(" ")
				b.WriteString(title)
			}
			b.WriteString("：")
		}
	}
	b.WriteString(desc)
	b.WriteString("\n操作者：")
	b.WriteString(actorLabelZH(ev.ActorKind, ev.Actor))
	return b.String()
}

// eventCardKeyTitle 从事件载荷取任务编号与标题。
func eventCardKeyTitle(payload string) (key, title string) {
	var p map[string]any
	if payload == "" {
		return "", ""
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return "", ""
	}
	if v, ok := p["key"].(string); ok {
		key = v
	}
	if v, ok := p["title"].(string); ok {
		title = v
	}
	return key, title
}

// actorLabelZH 把事件里的操作者翻成中文短语。
func actorLabelZH(kind, name string) string {
	switch ActorKind(kind) {
	case ActorHuman:
		return "人（" + name + "）"
	case ActorAgent:
		return "agent（" + name + "）"
	case ActorSystem:
		if strings.HasPrefix(name, automationActorPrefix) {
			return "自动化（" + strings.TrimPrefix(name, automationActorPrefix) + "）"
		}
		return "系统（" + name + "）"
	default:
		return name
	}
}
