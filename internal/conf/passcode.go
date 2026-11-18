package conf

import (
	"fmt"
	"log/slog"
)

const (
	// PasscodeMaxLen 是访问口令允许的最大长度，超过则拒绝启动。
	PasscodeMaxLen = 128
	// PasscodeWarnMinLen 短于该长度时启动只打 warn，不阻止服务（口令本身不会出现在日志里）。
	PasscodeWarnMinLen = 8
	// passcodeMinChar 和 passcodeMaxChar 是口令允许的字符范围：可打印的 ASCII，不含空格。
	passcodeMinChar = 0x21
	passcodeMaxChar = 0x7e
)

// PasscodeRequired 判断当前配置是否启用了访问口令。
func PasscodeRequired(bc *Bootstrap) bool {
	return bc != nil && bc.Server.HTTP.Passcode != ""
}

// ValidatePasscode 在启动时校验口令：长度不超过上限，且只能用英文字母、数字和常见符号。
// 限制字符是因为口令要放进 HTTP 请求头 `Authorization: Bearer <口令>`，
// 空格、中文和控制字符在不同客户端里会被截断或拒绝，配置成这样网页能登录而 MCP 连不上，
// 所以启动时直接报错，比运行时出现莫名其妙的 401 好排查。
func ValidatePasscode(bc *Bootstrap) error {
	if bc == nil {
		return nil
	}
	p := bc.Server.HTTP.Passcode
	if len(p) > PasscodeMaxLen {
		return fmt.Errorf("Server.HTTP.Passcode 长度不能超过 %d 个字符", PasscodeMaxLen)
	}
	for i := 0; i < len(p); i++ {
		if p[i] < passcodeMinChar || p[i] > passcodeMaxChar {
			return fmt.Errorf("Server.HTTP.Passcode 只能使用英文字母、数字和常见符号，不能含空格、中文或控制字符")
		}
	}
	return nil
}

// WarnShortPasscode 在口令偏短时打一条 warn，便于局域网共享时提醒用户加强口令。
func WarnShortPasscode(bc *Bootstrap) {
	if bc == nil {
		return
	}
	p := bc.Server.HTTP.Passcode
	if p != "" && len(p) < PasscodeWarnMinLen {
		slog.Warn("访问口令短于推荐长度", "recommend_min", PasscodeWarnMinLen)
	}
}
