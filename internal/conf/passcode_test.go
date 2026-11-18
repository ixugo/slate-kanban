package conf

import (
	"strings"
	"testing"
)

// passcodeConf 构造只带口令的配置，便于表驱动用例。
func passcodeConf(p string) *Bootstrap {
	return &Bootstrap{Server: Server{HTTP: ServerHTTP{Passcode: p}}}
}

// TestValidatePasscode 启动校验：留空表示不启用；超长、含空格/中文/控制字符的口令都要拒绝。
func TestValidatePasscode(t *testing.T) {
	cases := []struct {
		name     string
		passcode string
		wantErr  bool
	}{
		{"留空表示不启用", "", false},
		{"普通口令", "Slate-2026_ok!", false},
		{"刚好最大长度", strings.Repeat("x", PasscodeMaxLen), false},
		{"超过最大长度", strings.Repeat("x", PasscodeMaxLen+1), true},
		{"含空格", "has space", true},
		{"首尾空格", " padded ", true},
		{"含中文", "口令口令口令口令", true},
		{"含换行", "line\nbreak", true},
		{"含制表符", "tab\tchar", true},
		{"含 DEL 控制字符", "del\x7fchar", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePasscode(passcodeConf(tc.passcode))
			if (err != nil) != tc.wantErr {
				t.Fatalf("口令 %q: 期望出错=%v，实际 err=%v", tc.passcode, tc.wantErr, err)
			}
		})
	}
	if err := ValidatePasscode(nil); err != nil {
		t.Fatalf("空配置不应出错: %v", err)
	}
}

// TestPasscodeRequiredFlag 根据配置判断是否需要口令。
func TestPasscodeRequiredFlag(t *testing.T) {
	if PasscodeRequired(&Bootstrap{}) {
		t.Fatal("空配置不应需要口令")
	}
	if !PasscodeRequired(passcodeConf("x")) {
		t.Fatal("非空口令应需要")
	}
}
