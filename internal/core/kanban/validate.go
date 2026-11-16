package kanban

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// requireHuman 校验操作者合法并且是人。看板、列、规则的增删改只允许人操作，
// 所以这类方法都从它开始；agent 的权限被限制在 Core 里，而不是只靠界面隐藏入口。
func requireHuman(actor Actor) error {
	if err := actor.Validate(); err != nil {
		return err
	}
	if !actor.IsHuman() {
		return ErrHumanOnly
	}
	return nil
}

// checkText 校验一段文本：必须是合法的 UTF-8、不含空字符，字符数在 [min, max] 之内。
// 空字符和非法 UTF-8 在 PostgreSQL 里会让写入直接报错，在这里提前拦住，才能给出 400 而不是 500。
// field 是面向用户的字段名，会出现在错误提示里。
func checkText(field, s string, minLen, maxLen int) error {
	if !utf8.ValidString(s) {
		return invalidf("%s 含有非法字符", field)
	}
	if strings.ContainsRune(s, 0) {
		return invalidf("%s 不能包含空字符", field)
	}
	n := utf8.RuneCountInString(s)
	switch {
	case minLen > 0 && (n < minLen || n > maxLen):
		return invalidf("%s 长度必须在 %d 到 %d 个字符之间", field, minLen, maxLen)
	case n > maxLen:
		return invalidf("%s 不能超过 %d 个字符", field, maxLen)
	}
	return nil
}

// checkLine 在 checkText 的基础上要求是单行文本：不能有换行、制表等控制字符。
// 名称类字段在界面上只占一行，混进换行会把布局和日志都搞乱。
func checkLine(field, s string, minLen, maxLen int) error {
	if err := checkText(field, s, minLen, maxLen); err != nil {
		return err
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return invalidf("%s 不能包含换行等控制字符", field)
	}
	return nil
}
