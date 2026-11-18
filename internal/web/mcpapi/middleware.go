package mcpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"slate-kanban/internal/core/kanban"
)

// 身份写进请求头，工具从 req.Extra.Header 读取；键名固定，避免和客户端自定义头冲突。
const (
	headerActorKind = "X-Slate-Actor-Kind"
	headerActorName = "X-Slate-Actor-Name"
	headerViewRole  = "X-Slate-View-Role"
)

// namePattern 是 MCP ?name= 的格式：小写字母数字开头，后面可跟点、下划线、连字符，总长最多 64。
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// reservedNames 不允许 agent 使用的保留名。
var reservedNames = map[string]struct{}{
	"human":  {},
	"system": {},
}

// identityMiddleware 解析 ?name= 和 ?role=，校验后写进请求头（覆盖客户端自带的同名头）。
// 校验失败返回 400 和 JSON 错误体，不进入 MCP 协议处理。
func identityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, role, err := parseQueryIdentity(r.URL.Query())
		if err != nil {
			writeIdentityError(w, err)
			return
		}
		r.Header.Set(headerActorKind, string(actor.Kind))
		r.Header.Set(headerActorName, actor.Name)
		r.Header.Set(headerViewRole, role)
		next.ServeHTTP(w, r)
	})
}

// parseQueryIdentity 从查询参数得到看板操作者和视图角色。role 只接受空、worker、manager。
func parseQueryIdentity(q url.Values) (kanban.Actor, string, error) {
	rawRole := strings.TrimSpace(q.Get("role"))
	switch rawRole {
	case "", "worker":
		rawRole = "worker"
	case "manager":
	default:
		return kanban.Actor{}, "", errBadRole
	}
	name := strings.TrimSpace(q.Get("name"))
	if name == "" {
		if rawRole == "manager" {
			name = "manager"
		} else {
			name = "agent"
		}
	}
	if err := validateName(name); err != nil {
		return kanban.Actor{}, "", err
	}
	kind := kanban.ActorAgent
	if rawRole == "manager" {
		kind = kanban.ActorHuman
	}
	return kanban.Actor{Kind: kind, Name: name}, rawRole, nil
}

// validateName 检查 agent 自报名称的格式和保留名。
func validateName(name string) error {
	if !namePattern.MatchString(name) {
		return errBadName
	}
	if _, ok := reservedNames[name]; ok {
		return errReservedName
	}
	return nil
}

// identityError 是返回给调用方的 400 原因，带中文说明。
type identityError struct {
	msg string
}

func (e identityError) Error() string { return e.msg }

var (
	errBadName      = identityError{msg: "name 格式不合法：须以小写字母或数字开头，只能包含小写字母、数字、点、下划线和连字符，最长 64"}
	errReservedName = identityError{msg: "name 是保留名，不能使用 human 或 system"}
	errBadRole      = identityError{msg: "role 只接受 worker 或 manager"}
)

// writeIdentityError 把身份校验失败写成和普通 REST 相近的 JSON 错误体。
func writeIdentityError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"reason": "ErrBadRequest",
		"msg":    err.Error(),
	})
}
