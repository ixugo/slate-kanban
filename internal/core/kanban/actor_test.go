package kanban_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"slate-kanban/internal/core/kanban"
)

// TestActorValidate 验证操作者校验：未知类型、空名称、超长名称都被拒绝，合法的三种类型通过。
func TestActorValidate(t *testing.T) {
	tests := []struct {
		name  string
		actor kanban.Actor
		ok    bool
	}{
		{"人", kanban.Actor{Kind: kanban.ActorHuman, Name: "admin"}, true},
		{"agent", kanban.Actor{Kind: kanban.ActorAgent, Name: "claude-code"}, true},
		{"系统", kanban.Actor{Kind: kanban.ActorSystem, Name: "automation:开始计时"}, true},
		{"未知类型", kanban.Actor{Kind: "robot", Name: "x"}, false},
		{"空类型", kanban.Actor{Name: "x"}, false},
		{"空名称", kanban.Actor{Kind: kanban.ActorHuman}, false},
		{"名称刚好 64 个字符", kanban.Actor{Kind: kanban.ActorAgent, Name: strings.Repeat("字", 64)}, true},
		{"名称 65 个字符", kanban.Actor{Kind: kanban.ActorAgent, Name: strings.Repeat("字", 65)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.actor.Validate()
			if tt.ok && err != nil {
				t.Fatalf("应该通过，却返回: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("应该被拒绝，却通过了")
			}
			if !tt.ok && !errors.Is(err, kanban.ErrInvalidActor) {
				t.Fatalf("错误应该是 ErrInvalidActor，实际是 %v", err)
			}
		})
	}
}

// TestAutomationActorFitsLimit 验证自动化操作者的名称不会超过上限：
// 规则名很长时被截断，截断按字符而不是字节，所以汉字不会被切坏。
func TestAutomationActorFitsLimit(t *testing.T) {
	short := kanban.AutomationActor("开始计时")
	if short.Kind != kanban.ActorSystem || short.Name != "automation:开始计时" {
		t.Fatalf("短规则名不应被截断: %+v", short)
	}

	long := kanban.AutomationActor(strings.Repeat("长", 200))
	if n := utf8.RuneCountInString(long.Name); n != 64 {
		t.Fatalf("超长规则名应该截断到 64 个字符，实际 %d", n)
	}
	if !utf8.ValidString(long.Name) {
		t.Fatal("截断后出现了乱码")
	}
	if err := long.Validate(); err != nil {
		t.Fatalf("截断后的操作者应该合法: %v", err)
	}
}
