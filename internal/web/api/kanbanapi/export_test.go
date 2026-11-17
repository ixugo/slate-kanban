package kanbanapi

import "time"

// WithStreamTimingForTest 返回把实时流的心跳间隔和最长连接时间改短的副本，
// 测试用它，免得为了验证心跳和到期要真的等 20 秒、10 分钟。
func WithStreamTimingForTest(a API, heartbeat, maxAge time.Duration) API {
	a.heartbeat, a.maxAge = heartbeat, maxAge
	return a
}
