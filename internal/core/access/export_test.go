package access

import "time"

// 本文件只在测试时编译，把包内的未导出能力暴露给外部测试包 access_test。

// WithClockForTest 换掉 Core 使用的时钟，让测试可以精确控制"过了多久"。
func WithClockForTest(now func() time.Time) Option {
	return func(c *Core) { c.now = now }
}

// HashForTest 暴露令牌哈希的计算，用来在测试里核对库里存的是哈希而不是明文。
func HashForTest(token string) string { return hashToken(token) }
