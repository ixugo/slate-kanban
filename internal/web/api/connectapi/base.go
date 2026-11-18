package connectapi

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ixugo/goddd/pkg/reason"
)

const (
	// maxHostLen 是主机（含端口）的长度上限，和 DNS 对完整域名的长度限制一致。
	maxHostLen = 255
	// maxPort 是端口号的上限。
	maxPort = 65535
	// hostLabel 是域名里的一段（两个点之间）：字母、数字、下划线、连字符，不能以连字符开头或结尾。
	// 允许下划线，是因为容器编排里的服务名常常带下划线。
	hostLabel = `[A-Za-z0-9_](?:[A-Za-z0-9_-]*[A-Za-z0-9_])?`
)

var (
	// hostPattern 匹配合法的主机：域名或 IPv4（用点连接的若干段），或者带方括号的 IPv6，后面可以跟 :端口。
	// 分组 1 是端口。它只放行这些字符，所以引号、空格、斜杠、尖括号、@、分号这类能用来注入内容的字符都进不来。
	hostPattern = regexp.MustCompile(`^(?:` + hostLabel + `(?:\.` + hostLabel + `)*|\[[0-9A-Fa-f:.]+\])(?::([0-9]+))?$`)
	// errBadHost 是主机不合法时的错误，状态码 400。错误信息里不回显收到的主机，免得把攻击者的内容原样送回去。
	errBadHost = reason.ErrBadRequest.WithMsg("Host 头不合法，无法生成接入说明")
)

// baseURL 从请求里算出服务的对外地址（协议加主机），用来写进说明里的命令。
// 零配置：用户用什么地址打开页面，命令里就是什么地址。
//   - 协议：X-Forwarded-Proto 的第一个值（只认 http 和 https，别的忽略），没有就看连接是不是 TLS；
//   - 主机：X-Forwarded-Host 的第一个值，没有就用 Host。
//
// 这些头都是调用者自己发的，响应也只回给他自己，但地址会被写进用户要复制执行的命令，
// 所以主机必须通过严格校验，不合法就拒绝，而不是悄悄换成别的值。
func baseURL(r *http.Request) (string, error) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := strings.ToLower(firstValue(r.Header.Get("X-Forwarded-Proto"))); proto == "http" || proto == "https" {
		scheme = proto
	}
	host := firstValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if !validHost(host) {
		return "", errBadHost
	}
	return scheme + "://" + host, nil
}

// firstValue 取逗号分隔的请求头里的第一个值，去掉两端的空白。
// 经过多层代理时值会依次追加，第一个是离用户最近的那一层写的。
func firstValue(header string) string {
	first, _, _ := strings.Cut(header, ",")
	return strings.TrimSpace(first)
}

// validHost 判断主机（可以带端口）是否合法：长度不超过上限，字符和结构符合 hostPattern，端口在 1 到 65535 之间。
func validHost(host string) bool {
	if len(host) > maxHostLen {
		return false
	}
	match := hostPattern.FindStringSubmatch(host)
	if match == nil {
		return false
	}
	if match[1] == "" {
		return true
	}
	port, err := strconv.Atoi(match[1])
	return err == nil && port >= 1 && port <= maxPort
}
