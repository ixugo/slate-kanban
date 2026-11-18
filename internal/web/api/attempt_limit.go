package api

import (
	"sync"
	"time"
)

const (
	// maxFailedAttemptsPerWindow 同一 IP 在一个时间窗内允许记录的「错误凭证」次数上限。
	maxFailedAttemptsPerWindow = 10
	// failedAttemptWindow 统计错误凭证次数的固定窗口长度：从某个 IP 的第一次失败开始计时，
	// 窗口走完后次数清零，超限的 IP 也在窗口走完时解封。
	failedAttemptWindow = time.Minute
	// maxTrackedClientIPs 内存里最多跟踪的客户端 IP 数量，防止被大量伪造 IP 撑爆。
	// 表满时按「最早出现」淘汰，所以大量来源 IP 的分布式猜测可以挤掉旧记录；
	// 这只会让被挤掉的 IP 提前解封，不会影响内存上限。
	maxTrackedClientIPs = 1024
)

// attemptLimiter 按客户端 IP 统计错误口令尝试，超限时对该 IP 一律返回 429。
type attemptLimiter struct {
	mu    sync.Mutex
	now   func() time.Time
	ips   map[string]*clientAttempts
	order []string
}

// clientAttempts 记录某个 IP 在时间窗内的失败次数与窗口起点。
type clientAttempts struct {
	windowStart time.Time
	failures    int
	lastSeen    time.Time
}

// newAttemptLimiter 创建限流器，now 可注入便于测试时钟。
func newAttemptLimiter(now func() time.Time) *attemptLimiter {
	if now == nil {
		now = time.Now
	}
	return &attemptLimiter{
		now: now,
		ips: make(map[string]*clientAttempts),
	}
}

// recordFailure 在口令校验失败时记一次失败；返回当前是否已达封锁阈值。
func (l *attemptLimiter) recordFailure(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	rec := l.touchLocked(ip, now)
	rec.failures++
	rec.lastSeen = now
	if rec.windowStart.IsZero() || now.Sub(rec.windowStart) >= failedAttemptWindow {
		rec.windowStart = now
		rec.failures = 1
	}
	return rec.failures > maxFailedAttemptsPerWindow
}

// blocked 判断该 IP 是否仍在封锁期，并返回 Retry-After 秒数（向上取整）。
func (l *attemptLimiter) blocked(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	rec, ok := l.ips[ip]
	if !ok {
		return false, 0
	}
	if rec.failures <= maxFailedAttemptsPerWindow {
		return false, 0
	}
	elapsed := now.Sub(rec.windowStart)
	if elapsed >= failedAttemptWindow {
		delete(l.ips, ip)
		l.removeFromOrderLocked(ip)
		return false, 0
	}
	remain := failedAttemptWindow - elapsed
	sec := int(remain / time.Second)
	if remain%time.Second != 0 {
		sec++
	}
	if sec < 1 {
		sec = 1
	}
	return true, sec
}

// touchLocked 获取或新建 IP 记录，并在表满时淘汰过期或最旧的项。
func (l *attemptLimiter) touchLocked(ip string, now time.Time) *clientAttempts {
	if rec, ok := l.ips[ip]; ok {
		return rec
	}
	l.evictIfNeededLocked(now)
	rec := &clientAttempts{lastSeen: now}
	l.ips[ip] = rec
	l.order = append(l.order, ip)
	return rec
}

// evictIfNeededLocked 在跟踪数量达到上限时腾出一个位置：先删一个已过窗口的项，没有就淘汰最早出现的 IP。
// 时间复杂度 O(n)，n 最多 maxTrackedClientIPs（1024），而且只在表已满、又来了新 IP 时才会执行。
func (l *attemptLimiter) evictIfNeededLocked(now time.Time) {
	for len(l.ips) >= maxTrackedClientIPs {
		removed := false
		for k, rec := range l.ips {
			if now.Sub(rec.lastSeen) > failedAttemptWindow {
				delete(l.ips, k)
				l.removeFromOrderLocked(k)
				removed = true
				break
			}
		}
		if !removed && len(l.order) > 0 {
			oldest := l.order[0]
			delete(l.ips, oldest)
			l.order = l.order[1:]
		} else if !removed {
			break
		}
	}
}

// removeFromOrderLocked 从出现顺序切片里摘掉指定 IP。时间复杂度 O(n)，n 最多 1024。
func (l *attemptLimiter) removeFromOrderLocked(ip string) {
	for i, v := range l.order {
		if v == ip {
			l.order = append(l.order[:i], l.order[i+1:]...)
			return
		}
	}
}

// trackedCount 返回当前跟踪的 IP 数，仅供测试断言表规模。
func (l *attemptLimiter) trackedCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.ips)
}
