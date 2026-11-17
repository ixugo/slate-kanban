package kanban

import (
	"errors"
	"strings"
	"testing"
)

// TestCheckText 验证文本校验：长度按字符数而不是字节数算，空字符和非法 UTF-8 都要拒绝，
// 并且错误是 ErrInvalidInput，Web 层才会返回 400。
func TestCheckText(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		min, max int
		wantErr  bool
	}{
		{"必填但为空", "", 1, 5, true},
		{"必填刚好一个字符", "a", 1, 5, false},
		{"中文按字符数算", "一二三四五", 1, 5, false},
		{"中文超长一个字符", "一二三四五六", 1, 5, true},
		{"选填可以为空", "", 0, 5, false},
		{"选填超长", "abcdef", 0, 5, true},
		{"含空字符", "a\x00b", 0, 5, true},
		{"非法 UTF-8", "a\xffb", 0, 5, true},
		{"多行文本允许换行", "a\nb", 0, 5, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkText("字段", c.in, c.min, c.max)
			if (err != nil) != c.wantErr {
				t.Fatalf("checkText(%q) 的错误=%v，期望出错=%v", c.in, err, c.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("应该是 ErrInvalidInput，实际 %v", err)
			}
		})
	}
}

// TestCheckLine 验证单行文本不允许换行、制表等控制字符，并且错误里带着字段名。
func TestCheckLine(t *testing.T) {
	for _, in := range []string{"a\nb", "a\tb", "a\rb"} {
		err := checkLine("看板名称", in, 1, 10)
		if err == nil || !strings.Contains(err.Error(), "看板名称") {
			t.Fatalf("checkLine(%q) 应该拒绝并提到字段名，实际 %v", in, err)
		}
	}
	if err := checkLine("看板名称", "正常的名称", 1, 10); err != nil {
		t.Fatalf("正常名称不应该被拒绝: %v", err)
	}
}

// TestRequireHuman 验证只有合法的人能通过：agent 是 403，身份不合法是 401。
func TestRequireHuman(t *testing.T) {
	if err := requireHuman(Actor{Kind: ActorHuman, Name: "admin"}); err != nil {
		t.Fatalf("人应该通过: %v", err)
	}
	if err := requireHuman(Actor{Kind: ActorAgent, Name: "claude"}); !errors.Is(err, ErrHumanOnly) {
		t.Fatalf("agent 应该得到 ErrHumanOnly，实际 %v", err)
	}
	if err := requireHuman(Actor{Kind: ActorSystem, Name: "automation:x"}); !errors.Is(err, ErrHumanOnly) {
		t.Fatalf("系统操作者也不是人，应该得到 ErrHumanOnly，实际 %v", err)
	}
	if err := requireHuman(Actor{Kind: ActorHuman}); !errors.Is(err, ErrInvalidActor) {
		t.Fatalf("没有名称应该得到 ErrInvalidActor，实际 %v", err)
	}
}

// TestNormalizeLimit 验证数量整理：0 取默认值，超过上限取上限，正常值原样，负数返回 400。
func TestNormalizeLimit(t *testing.T) {
	cases := []struct {
		in, want int
		wantErr  bool
	}{
		{0, 100, false}, {1, 1, false}, {500, 500, false}, {501, 500, false}, {1 << 30, 500, false}, {-1, 0, true},
	}
	for _, c := range cases {
		got, err := normalizeLimit(c.in, 100, 500)
		if got != c.want || (err != nil) != c.wantErr || (err != nil && !errors.Is(err, ErrInvalidInput)) {
			t.Errorf("normalizeLimit(%d) = %d, %v，期望 %d，出错=%v", c.in, got, err, c.want, c.wantErr)
		}
	}
}
