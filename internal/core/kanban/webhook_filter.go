package kanban

import (
	"encoding/json"
	"slices"
)

// WebhookMatchesEventForTest 供测试直接调用订阅匹配逻辑。
func WebhookMatchesEventForTest(w *Webhook, ev *Event) bool {
	return webhookMatchesEvent(w, ev)
}

// webhookMatchesEvent 判断 webhook 是否应接收这条事件。
func webhookMatchesEvent(w *Webhook, ev *Event) bool {
	if !w.Enabled {
		return false
	}
	w.Normalize()
	if (w.strictMode(w.StrictFilterModes.EventTypes) || len(w.EventTypes) > 0) && !slices.Contains(w.EventTypes, ev.Type) {
		return false
	}
	if (w.strictMode(w.StrictFilterModes.ActorKinds) || len(w.ActorKinds) > 0) && !slices.Contains(w.ActorKinds, ev.ActorKind) {
		return false
	}
	if ev.Type == EventCardMoved && (w.strictMode(w.StrictFilterModes.MoveColumnIDs) || len(w.MoveColumnIDs) > 0) {
		toID := cardMovedToID(ev.Payload)
		if toID == 0 || !slices.Contains(w.MoveColumnIDs, toID) {
			return false
		}
	}
	return true
}

func (w *Webhook) strictMode(override *bool) bool {
	if override != nil {
		return *override
	}
	return w.StrictFilters
}

// cardMovedToID 从 card.moved 事件载荷读出目标列主键。
func cardMovedToID(payload string) int64 {
	var p map[string]any
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return 0
	}
	v, ok := p["to_id"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	default:
		return 0
	}
}
