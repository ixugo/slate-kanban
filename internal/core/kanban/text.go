package kanban

import "unicode/utf8"

// truncateRunes 按字符数截断字符串，不会把多字节的汉字切成乱码。
// 数据库字段的长度上限和接口校验都按字符数算，所以截断也必须按字符数。
func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
