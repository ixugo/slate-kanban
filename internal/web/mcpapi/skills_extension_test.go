package mcpapi_test

import (
	"crypto/sha256"
	json "encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"slate-kanban/internal/core/kanban"
	"slate-kanban/internal/core/kanban/kanbantest"
	"slate-kanban/internal/web/mcpapi"
	"slate-kanban/skills"
)

// 使用真实 Streamable HTTP Handler 与隔离数据库，不依赖网络监听端口。
func skillRPC(t *testing.T, handler http.Handler, session *string, method string, params any) map[string]any {
	t.Helper()
	reply, _ := skillRPCWire(t, handler, session, method, params)
	return reply
}

func skillRPCWire(t *testing.T, handler http.Handler, session *string, method string, params any) (map[string]any, string) {
	t.Helper()
	request := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if method != "notifications/initialized" {
		request["id"] = 1
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp?name=skills-test", strings.NewReader(string(raw)))
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	if *session != "" {
		req.Header.Set("Mcp-Session-Id", *session)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if id := res.Header().Get("Mcp-Session-Id"); id != "" {
		*session = id
	}
	if method == "notifications/initialized" {
		if res.Code != http.StatusAccepted {
			t.Fatalf("initialized: %d %s", res.Code, res.Body)
		}
		return nil, ""
	}
	if res.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", method, res.Code, res.Body)
	}
	var reply map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	return reply, res.Body.String()
}

// openSkillSession 用现有状态协议，测试结束时释放 SDK 会话。
func openSkillSession(t *testing.T, handler http.Handler, aware bool) (*string, map[string]any, string) {
	t.Helper()
	var session string
	caps := map[string]any{}
	if aware {
		caps["extensions"] = map[string]any{skills.ExtensionID: map[string]any{}}
	}
	reply, wire := skillRPCWire(t, handler, &session, "initialize", map[string]any{
		"protocolVersion": "2025-06-18", "capabilities": caps, "clientInfo": map[string]any{"name": "test", "version": "0"},
	})
	init := reply["result"].(map[string]any)
	if session == "" {
		t.Fatal("必须保留有状态 MCP 会话")
	}
	skillRPC(t, handler, &session, "notifications/initialized", map[string]any{})
	t.Cleanup(func() {
		req := httptest.NewRequest(http.MethodDelete, "/mcp?name=skills-test", nil)
		req.Header.Set("Mcp-Session-Id", session)
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	})
	return &session, init, wire
}

func rpcResult(t *testing.T, handler http.Handler, session *string, method string, params any) map[string]any {
	t.Helper()
	reply := skillRPC(t, handler, session, method, params)
	if reply["error"] != nil {
		t.Fatalf("%s: %v", method, reply)
	}
	return reply["result"].(map[string]any)
}

func findSkill(t *testing.T, result map[string]any, name string) map[string]any {
	t.Helper()
	for _, item := range items(t, result, "skills") {
		if item["uri"] == skills.URI(name) {
			return item
		}
	}
	t.Fatalf("目录中没有 %s: %v", name, result)
	return nil
}

func readSkillResource(t *testing.T, handler http.Handler, session *string, uri string) string {
	t.Helper()
	res := rpcResult(t, handler, session, "resources/read", map[string]any{"uri": uri})
	content := items(t, res, "contents")
	if len(content) != 1 || content[0]["uri"] != uri {
		t.Fatalf("资源响应错误: %v", res)
	}
	return content[0]["text"].(string)
}

func checkSkillManifest(t *testing.T, handler http.Handler, session *string, entry map[string]any) string {
	t.Helper()
	uri := entry["uri"].(string)
	text := readSkillResource(t, handler, session, uri)
	files := items(t, entry, "resources")
	if len(files) != 1 || files[0]["uri"] != uri || files[0]["digest"] != fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(text))) || files[0]["size"] != float64(len(text)) {
		t.Fatalf("manifest 与实际资源不一致: %v", entry)
	}
	head, _, ok := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	var front map[string]any
	if !ok {
		t.Fatal("缺少 YAML frontmatter")
	}
	if err := yaml.Unmarshal([]byte(head), &front); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(front, entry["frontmatter"]) {
		t.Fatalf("frontmatter 目录与文件不一致: %v / %v", front, entry["frontmatter"])
	}
	return text
}

func TestSkillsExtensionDiscovery(t *testing.T) {
	core := kanbantest.NewCore(t)
	api, stop := mcpapi.New(core, "test")
	t.Cleanup(stop)
	handler := api.Handler()
	session, init, _ := openSkillSession(t, handler, true)
	caps := init["capabilities"].(map[string]any)
	extensions, ok := caps["extensions"].(map[string]any)
	if !ok || extensions[skills.ExtensionID] == nil || caps["resources"] == nil {
		t.Fatalf("未声明 Skills 扩展: %v", caps)
	}
	if !strings.Contains(init["instructions"].(string), skills.URI(skills.CoreName)) || strings.Contains(init["instructions"].(string), "短句，一句一事") {
		t.Fatalf("未按需披露初始化提示: %v", init)
	}
	list := rpcResult(t, handler, session, "skills/list", map[string]any{})
	entries := items(t, list, "skills")
	if len(entries) != len(skills.Builtins()) || list["resultType"] != nil {
		t.Fatalf("有状态旧协议目录不正确: %v", list)
	}
	for _, doc := range skills.Builtins() {
		entry := findSkill(t, list, doc.Name)
		if entry["body"] != nil || entry["text"] != nil {
			t.Fatalf("列出目录时泄露了正文: %v", entry)
		}
		get := rpcResult(t, handler, session, "skills/get", map[string]any{"uri": doc.URI()})
		if !reflect.DeepEqual(get["skill"], entry) {
			t.Fatalf("单条目录与列表不一致: %v", get)
		}
		if text := checkSkillManifest(t, handler, session, entry); text != doc.Markdown() {
			t.Fatal("内置 Skill 资源不正确")
		}
	}
	if len(skills.Builtins()[0].Markdown()) >= len(skills.SlateKanban())/2 {
		t.Fatal("核心 Skill 未显著精简")
	}
	for _, uri := range []string{"", "skill://missing/SKILL.md", "skill://slate-kanban/other.md", "skill://slate-kanban/SKILL.md?x=1", "skill://SLATE-KANBAN/SKILL.md"} {
		for _, method := range []string{"skills/get", "resources/read"} {
			reply := skillRPC(t, handler, session, method, map[string]any{"uri": uri})
			if e, ok := reply["error"].(map[string]any); !ok || e["code"] != float64(-32602) {
				t.Errorf("%s %q 应返回 Invalid params: %v", method, uri, reply)
			}
		}
	}
}

func TestSkillsSameSessionUpdateAndLegacy(t *testing.T) {
	ctx := t.Context()
	core := kanbantest.NewCore(t)
	api, stop := mcpapi.New(core, "test")
	t.Cleanup(stop)
	handler := api.Handler()
	native, _, _ := openSkillSession(t, handler, true)
	legacy, init, _ := openSkillSession(t, handler, false)
	if !strings.Contains(init["instructions"].(string), "slate://skills/slate-kanban") || !strings.Contains(init["instructions"].(string), "短句，一句一事") {
		t.Fatal("旧客户端初始化说明发生变化")
	}
	if got := readSkillResource(t, handler, legacy, "slate://skills/slate-kanban"); got != skills.SlateKanban() {
		t.Fatal("旧 Skill 全文发生变化")
	}
	for _, sess := range []*string{native, legacy} {
		tools := items(t, rpcResult(t, handler, sess, "tools/list", map[string]any{}), "tools")
		var names []string
		for _, tool := range tools {
			names = append(names, tool["name"].(string))
		}
		if len(names) != len(agentTools) {
			t.Fatalf("工作工具数变化: %v", names)
		}
		for _, name := range agentTools {
			if !slices.Contains(names, name) {
				t.Fatalf("工作工具缺失: %s", name)
			}
		}
	}
	board, err := core.CreateBoard(ctx, humanActor, kanban.CreateBoardInput{Name: "技能验证", Slug: "SKTEST", Description: "动态项目说明", Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	prompt := func(sess *string) string {
		result := rpcResult(t, handler, sess, "tools/call", map[string]any{"name": "get_board", "arguments": map[string]any{"board_id": board.ID}})
		return result["structuredContent"].(map[string]any)["watch_prompt"].(string)
	}
	full, short := prompt(legacy), prompt(native)
	if len(short) >= len(full)/2 || !strings.Contains(full, "slate://skills/slate-kanban") || !strings.Contains(short, skills.URI(skills.WatchName)) {
		t.Fatalf("未按会话披露守候提示词: %d / %d", len(full), len(short))
	}
	for _, want := range []string{"技能验证", "动态项目说明", "工程师", "进行中优先", "待办", "独立 worktree", "评论确认", "finish_card"} {
		if !strings.Contains(short, want) {
			t.Errorf("短提示缺少动态信息或必要约束 %q", want)
		}
	}
	in := kanban.SkillInput{Name: "true", Description: `场景: "测试" # 说明`, Body: "# 初版\n\n    Markdown 缩进\n"}
	created, err := core.CreateSkill(ctx, humanActor, in)
	if err != nil {
		t.Fatal(err)
	}
	entry := findSkill(t, rpcResult(t, handler, native, "skills/list", map[string]any{}), in.Name)
	if got := checkSkillManifest(t, handler, native, entry); got != created.Markdown() {
		t.Fatal("已连接会话无法读到新增 Skill")
	}
	var catalog map[string]any
	if err := json.Unmarshal([]byte(readSkillResource(t, handler, legacy, "slate://skills/catalog")), &catalog); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(findSkill(t, catalog, in.Name), entry) {
		t.Fatal("旧客户端目录与扩展不一致")
	}
	in.Body = "# 新正文"
	updated, err := core.UpdateSkill(ctx, humanActor, in.Name, in)
	if err != nil {
		t.Fatal(err)
	}
	fresh := rpcResult(t, handler, native, "skills/get", map[string]any{"uri": created.URI})["skill"].(map[string]any)
	if reflect.DeepEqual(fresh, entry) || checkSkillManifest(t, handler, native, fresh) != updated.Markdown() {
		t.Fatal("已有会话未看到更新后的摘要和正文")
	}
	if err := core.DeleteSkill(ctx, humanActor, in.Name); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"skills/get", "resources/read"} {
		reply := skillRPC(t, handler, native, method, map[string]any{"uri": created.URI})
		if e, ok := reply["error"].(map[string]any); !ok || e["code"] != float64(-32602) {
			t.Errorf("删除后 %s 仍能读取: %v", method, reply)
		}
	}
	for _, item := range items(t, rpcResult(t, handler, native, "skills/list", map[string]any{}), "skills") {
		if item["uri"] == created.URI {
			t.Fatal("删除后目录仍然存在")
		}
	}
}

func TestSkillsListPagination(t *testing.T) {
	core := kanbantest.NewCore(t)
	for i := range 101 {
		if _, err := core.CreateSkill(t.Context(), humanActor, kanban.SkillInput{Name: fmt.Sprintf("test-%03d", i), Description: "fixture", Body: "# fixture"}); err != nil {
			t.Fatal(err)
		}
	}
	api, stop := mcpapi.New(core, "test")
	t.Cleanup(stop)
	handler := api.Handler()
	session, _, _ := openSkillSession(t, handler, true)
	first := rpcResult(t, handler, session, "skills/list", map[string]any{})
	second := rpcResult(t, handler, session, "skills/list", map[string]any{"cursor": first["nextCursor"]})
	seen := map[string]bool{}
	for _, page := range []map[string]any{first, second} {
		for _, item := range items(t, page, "skills") {
			uri := item["uri"].(string)
			if seen[uri] {
				t.Fatalf("重复目录项: %s", uri)
			}
			seen[uri] = true
		}
	}
	if len(seen) != 101+len(skills.Builtins()) || second["nextCursor"] != nil {
		t.Fatalf("分页遗漏: %d", len(seen))
	}
	bad := skillRPC(t, handler, session, "skills/list", map[string]any{"cursor": "bad!"})
	if e, ok := bad["error"].(map[string]any); !ok || e["code"] != float64(-32602) {
		t.Fatalf("非法游标未拒绝: %v", bad)
	}
}

// TestSkillsWireMeasurement 采集同一份合成任务在旧、新会话实际 SDK JSON-RPC 回复。
// 不启动服务端口，也不读取用户数据。可用 docs/measure_mcp_tokens.py --snapshot 统计。
func TestSkillsWireMeasurement(t *testing.T) {
	ctx := t.Context()
	core := kanbantest.NewCore(t)
	board, err := core.CreateBoard(ctx, humanActor, kanban.CreateBoardInput{Name: "Skill measurement", Slug: "SKMEAS", Description: "项目说明 / Project rules", Locale: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	card, err := core.CreateCard(ctx, humanActor, board.ID, kanban.CreateCardInput{ColumnID: board.Columns[2].ID, Title: "校验交接 / Validate handoff", Description: "逐项检查交接记录、任务状态和租约。"})
	if err != nil {
		t.Fatal(err)
	}
	api, stop := mcpapi.New(core, "slate-161-measure")
	t.Cleanup(stop)
	handler := api.Handler()
	snapshot := map[string]map[string]string{}
	for _, mode := range []string{"legacy", "skills"} {
		aware := mode == "skills"
		session, _, initWire := openSkillSession(t, handler, aware)
		records := map[string]string{"initialize": initWire}
		snapshot[mode] = records
		call := func(label, method string, params any) {
			t.Helper()
			reply, wire := skillRPCWire(t, handler, session, method, params)
			if reply["error"] != nil {
				t.Fatalf("%s: %v", label, reply)
			}
			records[label] = wire
		}
		call("tools/list", "tools/list", map[string]any{})
		call("get_board", "tools/call", map[string]any{"name": "get_board", "arguments": map[string]any{"board_id": board.ID}})
		call("get_card", "tools/call", map[string]any{"name": "get_card", "arguments": map[string]any{"card_id": card.ID}})
		if !aware {
			call("legacy_skill", "resources/read", map[string]any{"uri": "slate://skills/slate-kanban"})
			continue
		}
		call("skills/list", "skills/list", map[string]any{})
		call("skills/get", "skills/get", map[string]any{"uri": skills.URI(skills.CoreName)})
		call("core_skill", "resources/read", map[string]any{"uri": skills.URI(skills.CoreName)})
		call("development_skill", "resources/read", map[string]any{"uri": skills.URI(skills.DevelopmentName)})
		call("writing_skill", "resources/read", map[string]any{"uri": skills.URI(skills.WritingName)})
	}
	if len(snapshot["skills"]["get_board"]) >= len(snapshot["legacy"]["get_board"]) {
		t.Fatal("守候提示没有减少 wire 体积")
	}
	if snapshot["skills"]["tools/list"] != snapshot["legacy"]["tools/list"] {
		t.Fatal("工具列表不应随 Skills 披露变化")
	}
	if path := os.Getenv("SLATE_SKILL_WIRE_SNAPSHOT"); path != "" {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
