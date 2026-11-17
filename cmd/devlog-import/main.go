// 命令 devlog-import 通过 MCP 管理视图，把开发记录（JSON 文件）录入 Slate 的看板，
// 用来做到"用 Slate 管理 Slate 自己的开发"，同时也是对 MCP 管理工具的一次真实使用。
//
// 用法：先启动 Slate，再运行
//
//	go run ./cmd/devlog-import -url http://127.0.0.1:7070 -file docs/devlog/records.json
//
// 重复运行是安全的：看板按 slug 查找，卡片按标题去重，已经存在的会跳过。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// agentName 是连接 MCP 时用的名字，也是导入的评论的作者。
	agentName = "devlog-import"
	// listLimit 是 list_cards 单次最多返回的卡片数；超过它就无法保证去重完整，所以直接报错。
	listLimit = 200
	// importTimeout 是整个导入的总超时，防止服务没响应时一直卡住。
	importTimeout = 2 * time.Minute
)

// records 是开发记录文件的内容：一个看板，加上要录入的卡片。
type records struct {
	Board boardRecord  `json:"board"`
	Cards []cardRecord `json:"cards"`
}

// boardRecord 是看板的创建参数，字段含义和 MCP 的 create_board 一致。
type boardRecord struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ProjectPath string `json:"project_path"`
	Template    string `json:"template"`
	Locale      string `json:"locale"`
}

// cardRecord 是一张要录入的卡片，Column 写列的名称，其余字段含义和管理视图的 create_card 一致。
type cardRecord struct {
	Column      string   `json:"column"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Tags        []string `json:"tags"`
	DueAt       string   `json:"due_at"`
	StartedAt   string   `json:"started_at"`
	FinishedAt  string   `json:"finished_at"`
	Branch      string   `json:"branch"`
	URL         string   `json:"url"`
	Comments    []string `json:"comments"`
}

// importer 保存一次导入要用的 MCP 会话。
type importer struct {
	ctx  context.Context
	sess *mcp.ClientSession
}

// main 解析参数并执行导入，出错时把原因写到标准错误并以非零状态退出。
func main() {
	base := flag.String("url", "http://127.0.0.1:7070", "Slate 服务地址")
	file := flag.String("file", "docs/devlog/records.json", "开发记录文件")
	flag.Parse()
	if err := run(*base, *file); err != nil {
		fmt.Fprintln(os.Stderr, "导入失败:", err)
		os.Exit(1)
	}
}

// run 读取记录、连上 MCP、确保看板存在，然后逐张录入卡片并打印结果。
func run(base, file string) error {
	rec, err := loadRecords(file)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), importTimeout)
	defer cancel()
	imp, err := connect(ctx, base)
	if err != nil {
		return err
	}
	defer func() {
		if err := imp.sess.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "关闭 MCP 会话:", err)
		}
	}()

	boardID, columns, err := imp.ensureBoard(rec.Board)
	if err != nil {
		return err
	}
	created, skipped, err := imp.importCards(boardID, columns, rec.Cards)
	if err != nil {
		return err
	}
	fmt.Printf("看板 %s（id=%d）：新增 %d 张卡片，跳过 %d 张已存在的\n", rec.Board.Slug, boardID, created, skipped)
	return nil
}

// loadRecords 读取并校验记录文件：看板名称和标识必填，每张卡片都要有标题和列。
func loadRecords(path string) (*records, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取记录文件: %w", err)
	}
	var rec records
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, fmt.Errorf("解析记录文件 %s: %w", path, err)
	}
	if strings.TrimSpace(rec.Board.Name) == "" || strings.TrimSpace(rec.Board.Slug) == "" {
		return nil, fmt.Errorf("记录文件的 board.name 和 board.slug 不能为空")
	}
	seen := map[string]bool{}
	for i, c := range rec.Cards {
		if strings.TrimSpace(c.Title) == "" || strings.TrimSpace(c.Column) == "" {
			return nil, fmt.Errorf("第 %d 张卡片缺少 title 或 column", i+1)
		}
		if seen[c.Title] {
			return nil, fmt.Errorf("卡片标题重复，无法按标题去重: %q", c.Title)
		}
		seen[c.Title] = true
	}
	return &rec, nil
}

// connect 用管理视图连上 MCP 服务。
func connect(ctx context.Context, base string) (*importer, error) {
	endpoint := strings.TrimRight(base, "/") + "/mcp?" + url.Values{"name": {agentName}, "role": {"manager"}}.Encode()
	client := mcp.NewClient(&mcp.Implementation{Name: agentName, Version: "1"}, nil)
	sess, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint}, nil)
	if err != nil {
		return nil, fmt.Errorf("连接 MCP %s: %w", endpoint, err)
	}
	return &importer{ctx: ctx, sess: sess}, nil
}

// call 调用一个 MCP 工具，把结构化结果解码到 out（out 为 nil 时忽略结果）。
// 工具返回错误时，把服务端给的说明带进错误信息，方便定位是哪一条记录有问题。
func (im *importer) call(tool string, args map[string]any, out any) error {
	res, err := im.sess.CallTool(im.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return fmt.Errorf("调用 %s: %w", tool, err)
	}
	if res.IsError {
		return fmt.Errorf("工具 %s 返回错误: %s", tool, textOf(res))
	}
	if out == nil {
		return nil
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return fmt.Errorf("编码 %s 的结果: %w", tool, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析 %s 的结果: %w", tool, err)
	}
	return nil
}

// textOf 取出工具结果里的文字说明。
func textOf(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(t.Text)
		}
	}
	return sb.String()
}

// boardInfo 是 list_boards、create_board、get_board 结果里这里要用的字段。
type boardInfo struct {
	ID      int64  `json:"id"`
	Slug    string `json:"slug"`
	Columns []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"columns"`
}

// ensureBoard 按 slug 找到看板，没有就创建；返回看板编号和"列名到列编号"的对照。
func (im *importer) ensureBoard(b boardRecord) (int64, map[string]int64, error) {
	var list struct {
		Boards []boardInfo `json:"boards"`
	}
	if err := im.call("list_boards", nil, &list); err != nil {
		return 0, nil, err
	}
	id := int64(0)
	for _, existing := range list.Boards {
		if existing.Slug == b.Slug {
			id = existing.ID
		}
	}
	if id == 0 {
		var created boardInfo
		args := map[string]any{
			"name": b.Name, "slug": b.Slug, "description": b.Description,
			"project_path": b.ProjectPath, "template": b.Template, "locale": b.Locale,
		}
		if err := im.call("create_board", args, &created); err != nil {
			return 0, nil, err
		}
		id = created.ID
	}
	var detail boardInfo
	if err := im.call("get_board", map[string]any{"board_id": id}, &detail); err != nil {
		return 0, nil, err
	}
	columns := make(map[string]int64, len(detail.Columns))
	for _, c := range detail.Columns {
		columns[c.Name] = c.ID
	}
	return id, columns, nil
}

// existingTitles 返回看板里已有卡片的标题集合，用来跳过已经录入过的记录。
func (im *importer) existingTitles(boardID int64) (map[string]bool, error) {
	var list struct {
		Cards []struct {
			Title string `json:"title"`
		} `json:"cards"`
		Truncated bool `json:"truncated"`
	}
	if err := im.call("list_cards", map[string]any{"board_id": boardID, "limit": listLimit}, &list); err != nil {
		return nil, err
	}
	if list.Truncated {
		return nil, fmt.Errorf("看板卡片超过 %d 张，无法按标题去重", listLimit)
	}
	titles := make(map[string]bool, len(list.Cards))
	for _, c := range list.Cards {
		titles[c.Title] = true
	}
	return titles, nil
}

// importCards 按记录顺序录入卡片：已存在的标题跳过，其余创建并写上评论。返回新增和跳过的数量。
func (im *importer) importCards(boardID int64, columns map[string]int64, cards []cardRecord) (int, int, error) {
	titles, err := im.existingTitles(boardID)
	if err != nil {
		return 0, 0, err
	}
	created, skipped := 0, 0
	for _, c := range cards {
		if titles[c.Title] {
			skipped++
			continue
		}
		columnID, ok := columns[c.Column]
		if !ok {
			return created, skipped, fmt.Errorf("卡片 %q 指定的列 %q 在看板里不存在", c.Title, c.Column)
		}
		if err := im.createCard(boardID, columnID, c); err != nil {
			return created, skipped, fmt.Errorf("录入卡片 %q: %w", c.Title, err)
		}
		created++
	}
	return created, skipped, nil
}

// cardArgs 把一条卡片记录换成 create_card 的入参。空值一律不传，让服务端用默认值，
// 避免把 null 或空字符串当成"要清除"。
func cardArgs(boardID, columnID int64, c cardRecord) map[string]any {
	args := map[string]any{"board_id": boardID, "column_id": columnID, "title": c.Title}
	for key, value := range map[string]string{
		"description": c.Description, "branch": c.Branch, "url": c.URL,
		"due_at": c.DueAt, "started_at": c.StartedAt, "finished_at": c.FinishedAt,
	} {
		if value != "" {
			args[key] = value
		}
	}
	if len(c.Tags) > 0 {
		args["tags"] = c.Tags
	}
	if c.Priority != 0 {
		args["priority"] = c.Priority
	}
	return args
}

// createCard 创建一张卡片，并把记录里的评论依次写上去。
func (im *importer) createCard(boardID, columnID int64, c cardRecord) error {
	args := cardArgs(boardID, columnID, c)
	var card struct {
		ID int64 `json:"id"`
	}
	if err := im.call("create_card", args, &card); err != nil {
		return err
	}
	for _, body := range c.Comments {
		if err := im.call("comment_card", map[string]any{"card_id": card.ID, "body": body}, nil); err != nil {
			return fmt.Errorf("写评论: %w", err)
		}
	}
	return nil
}
