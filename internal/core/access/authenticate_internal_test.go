package access

import (
	"strings"
	"testing"
)

// TestParseBearer 验证请求头解析：只认 Bearer 加令牌，Bearer 不区分大小写，令牌前后的空白忽略；
// 其他方案、缺令牌、令牌里夹空白、超过长度上限都算格式不对。
func TestParseBearer(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{"标准写法", "Bearer abc", "abc", true},
		{"小写 bearer", "bearer abc", "abc", true},
		{"大写 BEARER", "BEARER abc", "abc", true},
		{"前后有空白", "  Bearer   abc  ", "abc", true},
		{"刚好是长度上限", "Bearer " + strings.Repeat("a", maxBearerLen), strings.Repeat("a", maxBearerLen), true},
		{"超过长度上限", "Bearer " + strings.Repeat("a", maxBearerLen+1), "", false},
		{"空字符串", "", "", false},
		{"只有方案", "Bearer", "", false},
		{"方案后面全是空白", "Bearer    ", "", false},
		{"别的方案", "Basic abc", "", false},
		{"没有方案", "abc", "", false},
		{"令牌里夹空格", "Bearer a b", "", false},
		{"令牌里夹制表符", "Bearer a\tb", "", false},
	}
	for _, c := range cases {
		got, ok := parseBearer(c.header)
		if got != c.want || ok != c.ok {
			t.Errorf("%s：parseBearer(%q) = %q, %v，期望 %q, %v", c.name, c.header, got, ok, c.want, c.ok)
		}
	}
}
