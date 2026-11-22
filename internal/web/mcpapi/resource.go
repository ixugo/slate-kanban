package mcpapi

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/skills"
)

const (
	// maxResourceCards 是看板资源快照里最多给出的任务摘要数，和 list_cards 的单次上限分开，
	// 资源是订阅和整体了解看板用的，不是每次干活都读。
	maxResourceCards = 200
	// boardURIPrefix 是看板资源地址的固定前缀，后面跟看板编号。
	boardURIPrefix = "slate://boards/"
	// skillURI 是 slate-kanban skill 资源的固定地址。
	skillURI = "slate://skills/slate-kanban"
	// markdownMIME 是 skill 资源的内容类型。
	markdownMIME = "text/markdown"
)

// boardResource 是读取看板资源得到的内容：看板、列、规则，任务只有摘要，加上读取数据之前的事件序号 seq。
// 任务数有上限，多了只给前面的，total 和 truncated 如实说明，要看别的任务用 list_cards 的筛选。
type boardResource struct {
	*kanban.BoardDetail
	Cards     []cardSummary `json:"cards"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`
	Seq       int64         `json:"seq"`
}

// boardURI 生成一个看板的资源地址。
func boardURI(id int64) string {
	return boardURIPrefix + strconv.FormatInt(id, 10)
}

// parseBoardURI 从资源地址里取出看板编号。解析很严格：必须是固定前缀加上不带前导零的正整数，
// 不允许符号、多余的路径、查询参数和空白，解析不了就当作资源不存在，读取和订阅用同一套规则。
func parseBoardURI(uri string) (int64, bool) {
	rest, found := strings.CutPrefix(uri, boardURIPrefix)
	if !found || rest == "" || rest[0] == '0' {
		return 0, false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	return id, err == nil
}

// registerResources 注册看板快照模板、skill 和 Python 监听脚本。
func (a API) registerResources(s *mcp.Server) {
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:  "board",
		Title: "看板快照",
		Description: "一个看板的当前快照：列、规则和任务摘要（最多 200 个，没有描述和评论），以及读取时的事件序号 seq。" +
			"订阅它，看板有更新时会收到通知；平时用 pull_updates 取增量，用 get_card 看任务全文。",
		MIMEType:    "application/json",
		URITemplate: boardURITemplate,
	}, a.readBoard)
	s.AddResource(&mcp.Resource{
		Name:  "skill",
		Title: "slate-kanban skill",
		Description: "agent 使用这个看板的完整流程：确认项目路径、守候、领取、用子 agent 在 worktree 里开发、交接到审查。" +
			"还没装这个 skill 时读取它，经用户同意后存成 skills 目录里的 slate-kanban/SKILL.md。",
		MIMEType: markdownMIME,
		URI:      skillURI,
	}, a.readSkill)
	s.AddResource(&mcp.Resource{
		Name:        "slate-watch",
		Title:       "Slate 的 Python 看板监听器",
		Description: "空闲时阻塞监听，收到事件才启动本机已选的调度客户端。",
		MIMEType:    "text/x-python",
		URI:         skills.WatchURI,
	}, a.readWatchScript)
	s.AddResource(&mcp.Resource{
		Name:        "codex-watch",
		Title:       "Slate 的 Python 看板监听器（原资源地址）",
		Description: "与 slate://scripts/slate-watch.py 相同。",
		MIMEType:    "text/x-python",
		URI:         skills.CodexWatchURI,
	}, a.readWatchScript)
}

// readSkill 返回编译进服务的 skill 全文。skill 和服务端是同一个版本，所以里面提到的工具一定存在。
func (a API) readSkill(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: skillURI, MIMEType: markdownMIME, Text: skills.SlateKanban()},
	}}, nil
}

func (a API) readWatchScript(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: req.Params.URI, MIMEType: "text/x-python", Text: skills.WatchScript()},
	}}, nil
}

// readBoard 读取看板资源，返回整板快照。
func (a API) readBoard(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI
	id, found := parseBoardURI(uri)
	if !found {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	snap, err := a.core.BoardSnapshot(ctx, id)
	if err != nil {
		return nil, resourceError(ctx, uri, err)
	}
	body, err := json.Marshal(boardResourceOf(snap))
	if err != nil {
		return nil, resourceError(ctx, uri, err)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
		{URI: uri, MIMEType: "application/json", Text: string(body)},
	}}, nil
}

// boardResourceOf 把整板快照缩成资源内容：任务换成摘要，数量最多 maxResourceCards 张。
func boardResourceOf(snap *kanban.Snapshot) boardResource {
	cards := snap.Cards
	total := len(cards)
	if total > maxResourceCards {
		cards = cards[:maxResourceCards]
	}
	return boardResource{
		BoardDetail: snap.BoardDetail, Cards: summarizeAll(cards),
		Total: total, Truncated: total > maxResourceCards, Seq: snap.Seq,
	}
}

// subscribe 处理订阅请求：地址必须是合法的看板资源地址，并且看板存在。
// 任何已认证的身份都可以订阅，因为读看板本来就对所有身份开放。
func (a API) subscribe(ctx context.Context, req *mcp.SubscribeRequest) error {
	uri := req.Params.URI
	id, found := parseBoardURI(uri)
	if !found {
		return mcp.ResourceNotFoundError(uri)
	}
	if _, err := a.core.GetBoard(ctx, id); err != nil {
		return resourceError(ctx, uri, err)
	}
	return nil
}

// unsubscribe 处理取消订阅请求。订阅记录由 SDK 维护，这里没有额外的东西要清理。
func (a API) unsubscribe(context.Context, *mcp.UnsubscribeRequest) error {
	return nil
}

// resourceError 把读取或订阅资源时 Core 返回的错误换成协议错误：看板不存在就是资源不存在，
// 其他错误和工具错误一样处理，只把 reason 和 msg 给调用方，细节留在日志里。
func resourceError(ctx context.Context, uri string, err error) error {
	if errors.Is(err, kanban.ErrBoardNotFound) {
		return mcp.ResourceNotFoundError(uri)
	}
	return toolError(ctx, err)
}
