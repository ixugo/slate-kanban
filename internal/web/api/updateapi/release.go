package updateapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// cacheTTL 是最新版本的缓存时间：网页每次打开都去问 GitHub 太浪费，也怕被网站的访问频率限制拦住。
	cacheTTL = 10 * time.Second
	// failTTL 是查询失败后不再重试的时间，GitHub 不通时不用每次打开网页都等一次超时。
	failTTL = 5 * time.Minute
	// maxFeedBytes 是 release 订阅源响应体的大小上限。
	maxFeedBytes = 1 << 20
)

// tagPattern 限定 tag 只能是安全字符：它会被拼进包名、日志和提示里。
var tagPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// asset 是 release 里确认存在的一个文件。
type asset struct {
	Name string
	URL  string
}

// release 是最新发布的信息：版本号来自 releases/latest 的跳转，说明来自订阅源，
// 安装包是否已经上传则逐个探测下载地址得知。这些都是 github.com 网页地址，没有 API 的匿名额度限制。
type release struct {
	Tag    string
	Body   string
	URL    string
	Assets []asset
}

// packagesReady 判断当前平台的包和校验文件是否都已上传：发布工作流先建 release、后传包，
// 以包真实存在为准，比按时间猜更准，包一传完就能提示和更新，不用多等。
func (u *updater) packagesReady(rel *release) bool {
	return u.assetURL(rel, u.archiveName(rel.Tag)) != "" && u.assetURL(rel, checksumsName) != ""
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
	u.cached, u.cachedAt, u.failedAt = rel, time.Now(), time.Time{}
	return rel, nil
}

// fetchLatest 汇总最新 release：先问版本号，再读订阅源里的更新时间和说明，最后探测安装包是否已上传。
func (u *updater) fetchLatest(ctx context.Context) (*release, error) {
	tag, err := u.latestTag(ctx)
	if err != nil {
		return nil, err
	}
	rel := &release{Tag: tag, URL: u.opt.WebBase + "/" + repo + "/releases/tag/" + tag}
	if err := u.fillFromFeed(ctx, rel); err != nil {
		return nil, err
	}
	if err := u.fillAssets(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// latestTag 请求 releases/latest 并读它的跳转地址，得到最新版本的 tag；不跟随跳转，只取 Location。
func (u *updater) latestTag(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.opt.WebBase+"/"+repo+"/releases/latest", http.NoBody)
	if err != nil {
		return "", err
	}
	client := *u.opt.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 300 || res.StatusCode >= 400 {
		return "", fmt.Errorf("GitHub 返回 %d，可能还没有发布过 release", res.StatusCode)
	}
	_, tag, ok := strings.Cut(res.Header.Get("Location"), "/releases/tag/")
	if !ok || !tagPattern.MatchString(tag) {
		return "", fmt.Errorf("release 的 tag 不合法")
	}
	return tag, nil
}

// feed 是 releases.atom 里用得到的部分。
type feed struct {
	Entries []struct {
		Link struct {
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Content string `xml:"content"`
	} `xml:"entry"`
}

// htmlTag 匹配 HTML 标签，订阅源里的说明是 HTML，网页只显示文字。
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// blankLines 匹配连续三个以上的换行，说明里的段落间只保留一个空行。
var blankLines = regexp.MustCompile(`\n{3,}`)

// fillFromFeed 从订阅源找到这个 tag 的条目，取说明文字。
func (u *updater) fillFromFeed(ctx context.Context, rel *release) error {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.opt.WebBase+"/"+repo+"/releases.atom", http.NoBody)
	if err != nil {
		return err
	}
	res, err := u.opt.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("读取 release 订阅源失败：GitHub 返回 %d", res.StatusCode)
	}
	var f feed
	if err := xml.NewDecoder(io.LimitReader(res.Body, maxFeedBytes)).Decode(&f); err != nil {
		return fmt.Errorf("解析 release 订阅源: %w", err)
	}
	for _, e := range f.Entries {
		if strings.HasSuffix(e.Link.Href, "/releases/tag/"+rel.Tag) {
			rel.Body = releaseNotes(e.Content)
			return nil
		}
	}
	return fmt.Errorf("订阅源里没有 %s", rel.Tag)
}

// releaseNotes 把订阅源里的 HTML 说明转成纯文字：段落之间留空行，标签去掉，实体还原；GitHub 没有说明时给的占位文字当作空。
func releaseNotes(content string) string {
	text := html.UnescapeString(content)
	text = strings.NewReplacer("</p>", "\n\n", "<br>", "\n", "<br/>", "\n", "</li>", "\n").Replace(text)
	text = html.UnescapeString(htmlTag.ReplaceAllString(text, ""))
	text = strings.TrimSpace(blankLines.ReplaceAllString(text, "\n\n"))
	if text == "No content." {
		return ""
	}
	return text
}

// fillAssets 探测当前平台的安装包和校验文件是否已经上传：发请求问下载地址，200 才算存在。
func (u *updater) fillAssets(ctx context.Context, rel *release) error {
	for _, name := range []string{u.archiveName(rel.Tag), checksumsName} {
		url := u.opt.DownloadPrefix + rel.Tag + "/" + name
		ok, err := u.exists(ctx, url)
		if err != nil {
			return err
		}
		if ok {
			rel.Assets = append(rel.Assets, asset{Name: name, URL: url})
		}
	}
	return nil
}

// exists 用 HEAD 探测一个地址是否存在（跟随跳转，GitHub 的下载地址会跳到对象存储）。
func (u *updater) exists(ctx context.Context, url string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, http.NoBody)
	if err != nil {
		return false, err
	}
	res, err := u.opt.Client.Do(req)
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK, nil
}
