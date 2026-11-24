package mcpapi

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

const (
	// 当前 SDK 的有状态传输使用 2025-11-25 及以前的协议。SEP-2640 的兼容层
	// 使用当时的结果信封（无 resultType 和缓存字段），不改变会话协议。
	skillListMethod  = "skills/list"
	skillGetMethod   = "skills/get"
	skillPageSize    = 100
	skillCatalogURI  = "slate://skills/catalog"
	skillURITemplate = skills.URIPrefix + "{name}" + skills.URISuffix
)

// skillEntry 是 SEP-2640 的目录项。frontmatter 与 SKILL.md 的 YAML 完全对应，
// manifest 列出单个文件的原始字节数和内容摘要，不包含正文。
type skillEntry struct {
	URI         string           `json:"uri"`
	Frontmatter skillFrontmatter `json:"frontmatter"`
	Resources   []skillFile      `json:"resources"`
}

type skillFrontmatter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type skillFile struct {
	URI    string `json:"uri"`
	Digest string `json:"digest"`
	Size   int    `json:"size"`
}

type skillsListParams struct {
	mcp.ParamsBase
	Cursor string `json:"cursor,omitempty"`
}

type skillsListResult struct {
	mcp.ResultBase
	Skills     []skillEntry `json:"skills"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

type skillsGetParams struct {
	mcp.ParamsBase
	URI string `json:"uri"`
}

type skillsGetResult struct {
	mcp.ResultBase
	Skill skillEntry `json:"skill"`
}

func skillEntryOf(item kanban.SkillSummary) skillEntry {
	return skillEntry{
		URI:         item.URI,
		Frontmatter: skillFrontmatter{Name: item.Name, Description: item.Description},
		Resources:   []skillFile{{URI: item.URI, Digest: item.Digest, Size: item.Size}},
	}
}

func skillEntries(items []kanban.SkillSummary) []skillEntry {
	out := make([]skillEntry, 0, len(items))
	for _, item := range items {
		out = append(out, skillEntryOf(item))
	}
	return out
}

func parseSkillURI(uri string) (string, bool) {
	name, ok := strings.CutPrefix(uri, skills.URIPrefix)
	if !ok {
		return "", false
	}
	name, ok = strings.CutSuffix(name, skills.URISuffix)
	return name, ok && kanban.ValidSkillName(name)
}

// registerSkills 复用 SDK 的方法注册和资源读取，不额外创建 HTTP 入口。
// 资源模板和方法每次读当前目录，所以已建立的会话也能看到新增、编辑及删除。
func (a API) registerSkills(s *mcp.Server, v view) {
	if err := mcp.AddReceivingCustomMethod(s, skillListMethod, a.listSkills); err != nil {
		panic(err)
	}
	if err := mcp.AddReceivingCustomMethod(s, skillGetMethod, a.getSkill); err != nil {
		panic(err)
	}
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name: "skill-document", Title: "Skill Markdown",
		Description: "读取目录中指定 Skill 的 SKILL.md。",
		MIMEType:    markdownMIME, URITemplate: skillURITemplate,
	}, a.readSkillDocument)
	for _, doc := range skills.Builtins() {
		s.AddResource(&mcp.Resource{
			Name: doc.Name, Description: doc.Description, URI: doc.URI(), MIMEType: markdownMIME,
		}, a.readSkillDocument)
	}
	// 旧客户端不认识 skills/list 时，也能通过普通资源获取最新目录。
	s.AddResource(&mcp.Resource{
		Name: "skill-catalog", Title: "Skill 目录",
		Description: "内置及自定义 Skill 的名称、简介、URI 和文件摘要。按需读取正文。",
		MIMEType:    "application/json", URI: skillCatalogURI,
	}, a.readSkillCatalog)
	s.AddReceivingMiddleware(skillDisclosure(v))
}

func (a API) listSkills(ctx context.Context, _ *mcp.ServerSession, in *skillsListParams) (*skillsListResult, error) {
	var after string
	if in != nil && in.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(in.Cursor)
		if err != nil || !kanban.ValidSkillName(string(raw)) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid skill cursor"}
		}
		after = string(raw)
	}
	items, err := a.core.ListSkills(ctx)
	if err != nil {
		return nil, toolError(ctx, err)
	}
	start := sort.Search(len(items), func(i int) bool { return items[i].Name > after })
	end := min(start+skillPageSize, len(items))
	out := &skillsListResult{Skills: skillEntries(items[start:end])}
	if end < len(items) {
		out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(items[end-1].Name))
	}
	return out, nil
}

func (a API) getSkill(ctx context.Context, _ *mcp.ServerSession, in *skillsGetParams) (*skillsGetResult, error) {
	if in == nil {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "missing skill URI"}
	}
	name, ok := parseSkillURI(in.URI)
	if !ok {
		return nil, mcp.ResourceNotFoundError(in.URI)
	}
	item, err := a.core.GetSkillMetadata(ctx, name)
	if err != nil {
		return nil, skillResourceError(ctx, in.URI, err)
	}
	return &skillsGetResult{Skill: skillEntryOf(*item)}, nil
}

func (a API) skillByURI(ctx context.Context, uri string) (*kanban.SkillDetail, error) {
	name, ok := parseSkillURI(uri)
	if !ok {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	item, err := a.core.GetSkill(ctx, name)
	if err != nil {
		return nil, skillResourceError(ctx, uri, err)
	}
	return item, nil
}

func skillResourceError(ctx context.Context, uri string, err error) error {
	if errors.Is(err, kanban.ErrSkillNotFound) {
		return mcp.ResourceNotFoundError(uri)
	}
	return toolError(ctx, err)
}

func (a API) readSkillDocument(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	item, err := a.skillByURI(ctx, req.Params.URI)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: item.URI, MIMEType: markdownMIME, Text: item.Markdown()},
	}}, nil
}

func (a API) readSkillCatalog(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	items, err := a.core.ListSkills(ctx)
	if err != nil {
		return nil, toolError(ctx, err)
	}
	body, err := json.Marshal(skillsListResult{Skills: skillEntries(items)})
	if err != nil {
		return nil, toolError(ctx, err)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: skillCatalogURI, MIMEType: "application/json", Text: string(body)},
	}}, nil
}

type skillsContextKey struct{}

func supportsSkills(caps *mcp.ClientCapabilities) bool {
	if caps == nil {
		return false
	}
	_, ok := caps.Extensions[skills.ExtensionID].(map[string]any)
	return ok
}

func skillsAware(ctx context.Context) bool {
	aware, _ := ctx.Value(skillsContextKey{}).(bool)
	return aware
}

// skillDisclosure 只对明确支持扩展的会话启用短提示词；旧客户端仍得到原始完整流程。
func skillDisclosure(v view) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if call, ok := req.(*mcp.CallToolRequest); ok && call.Params.Name == "get_board" && supportsSkills(call.ClientCapabilities()) {
				ctx = context.WithValue(ctx, skillsContextKey{}, true)
			}
			res, err := next(ctx, method, req)
			if method == "initialize" && err == nil {
				if params, ok := req.GetParams().(*mcp.InitializeParams); ok && supportsSkills(params.Capabilities) {
					res.(*mcp.InitializeResult).Instructions = skillInstructionsFor(v)
				}
			}
			return res, err
		}
	}
}
