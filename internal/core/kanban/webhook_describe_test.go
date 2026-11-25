package kanban_test

import (
	"testing"

	"slate-kanban/internal/core/kanban"
)

// TestDescribeEventZHReleased 验证领取被放开的事件按原因写出不同的句子：
// 主动释放、人收回、换列、租约到期各一句，没有原因时按主动释放处理。
func TestDescribeEventZHReleased(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"主动释放", `{"agent":"codex","why":"released"}`, "释放了任务"},
		{"没有原因", `{"agent":"codex"}`, "释放了任务"},
		{"人收回", `{"agent":"codex","why":"revoked"}`, "收回了 agent「codex」对任务的持有"},
		{"换列", `{"agent":"codex","why":"moved"}`, "任务换列，结束了 agent「codex」的持有"},
		{"租约到期", `{"agent":"codex","why":"expired","expired_at":"2026-11-17T12:00:00Z"}`, "发现 agent「codex」的租约已到期，领取结束"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := &kanban.Event{Type: kanban.EventCardReleased, Actor: "claude", ActorKind: string(kanban.ActorAgent), Payload: tc.payload}
			if got := kanban.DescribeEventZH(ev); got != tc.want {
				t.Fatalf("描述不对: 实际 %q，期望 %q", got, tc.want)
			}
		})
	}
}
