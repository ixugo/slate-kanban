package updateapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// cacheTTL 是最新版本的缓存时间：匿名访问 GitHub API 每小时只有 60 次，网页每次打开都问一次会被限流。
	cacheTTL = time.Hour
	// failTTL 是查询失败后不再重试的时间，GitHub 不通时不用每次打开网页都等一次超时。
	failTTL = 5 * time.Minute
	// maxReleaseBytes 是 release 信息响应体的大小上限。
	maxReleaseBytes = 1 << 20
)

// tagPattern 限定 tag 只能是安全字符：它会被拼进包名、日志和提示里。
var tagPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// asset 是 release 里的一个文件。
type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// release 是 GitHub 返回的最新发布。
type release struct {
	Tag    string  `json:"tag_name"`
	Body   string  `json:"body"`
	URL    string  `json:"html_url"`
	Assets []asset `json:"assets"`
}

// parseVersion 把 v1.2.3 或 1.2.3 解析成三段数字；后面带的预发布或构建信息（-rc1、+abc）直接忽略。
func parseVersion(s string) ([3]int, bool) {
	var out [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// newer 判断 latest 是否比 current 新；任何一个解析不了都按「不新」处理，宁可不提示也不乱提示。
func newer(latest, current string) bool {
	l, okL := parseVersion(latest)
	c, okC := parseVersion(current)
	if !okL || !okC {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// latest 返回最新的 release；force 为真时跳过缓存（更新前要拿最新的）。
// 持锁期间才去请求，所以并发打开网页只会发出一次请求；失败后 failTTL 内直接返回上次的错误。
func (u *updater) latest(ctx context.Context, force bool) (*release, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	if !force && u.cached != nil && now.Sub(u.cachedAt) < cacheTTL {
		return u.cached, nil
	}
	if !force && !u.failedAt.IsZero() && now.Sub(u.failedAt) < failTTL {
		return nil, fmt.Errorf("最近查询失败，稍后再试")
	}
	rel, err := u.fetchLatest(ctx)
	if err != nil {
		u.failedAt = now
		return nil, err
	}
	u.cached, u.cachedAt, u.failedAt = rel, now, time.Time{}
	return rel, nil
}

// fetchLatest 请求 GitHub 的 releases/latest，并校验 tag 的字符。
func (u *updater) fetchLatest(ctx context.Context) (*release, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.opt.APIBase+"/repos/"+repo+"/releases/latest", http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := u.opt.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 返回 %d", res.StatusCode)
	}
	var rel release
	if err := json.NewDecoder(io.LimitReader(res.Body, maxReleaseBytes)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析 release 信息: %w", err)
	}
	if !tagPattern.MatchString(rel.Tag) {
		return nil, fmt.Errorf("release 的 tag 不合法")
	}
	return &rel, nil
}
