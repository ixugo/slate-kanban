package api

import (
	"fmt"
	"testing"
	"time"
)

// TestAttemptLimiterBlocksAfterEleventhFailure 验证第 11 次失败后进入封锁，窗口结束后恢复。
func TestAttemptLimiterBlocksAfterEleventhFailure(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	l := newAttemptLimiter(clock)
	ip := "203.0.113.1"
	for i := 0; i < 10; i++ {
		if blocked, _ := l.blocked(ip); blocked {
			t.Fatalf("前 10 次错误不应封锁: 第 %d 次", i+1)
		}
		l.recordFailure(ip)
	}
	l.recordFailure(ip)
	if blocked, _ := l.blocked(ip); !blocked {
		t.Fatal("第 11 次错误后应封锁")
	}
	now = now.Add(failedAttemptWindow + time.Second)
	if blocked, _ := l.blocked(ip); blocked {
		t.Fatal("窗口过后应解除封锁")
	}
}

// TestAttemptLimiterIsPerIP 一个 IP 被封锁，不影响其他 IP。
func TestAttemptLimiterIsPerIP(t *testing.T) {
	l := newAttemptLimiter(func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) })
	for i := 0; i < maxFailedAttemptsPerWindow+1; i++ {
		l.recordFailure("203.0.113.1")
	}
	if blocked, _ := l.blocked("203.0.113.1"); !blocked {
		t.Fatal("猜错超限的 IP 应被封锁")
	}
	if blocked, _ := l.blocked("203.0.113.2"); blocked {
		t.Fatal("其他 IP 不应被连坐")
	}
}

// TestAttemptLimiterCapEvictsOldest 验证超过 maxTrackedClientIPs 时会淘汰记录。
func TestAttemptLimiterCapEvictsOldest(t *testing.T) {
	fixed := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	l := newAttemptLimiter(func() time.Time { return fixed })
	for i := range maxTrackedClientIPs + 5 {
		l.recordFailure(fmtIP(i))
	}
	if c := l.trackedCount(); c != maxTrackedClientIPs {
		t.Fatalf("跟踪数应正好停在上限 %d（说明确实淘汰过）: %d", maxTrackedClientIPs, c)
	}
}

// fmtIP 把序号编成互不相同的测试用伪 IP（序号不超过 65535）。
func fmtIP(i int) string {
	return fmt.Sprintf("10.0.%d.%d", i/256, i%256)
}
