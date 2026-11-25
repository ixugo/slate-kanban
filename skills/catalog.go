package skills

import (
	_ "embed"
	"slices"
	"strconv"
	"strings"
)

const (
	CoreName        = "slate-kanban"
	WatchName       = "slate-watch"
	DevelopmentName = "slate-development"
	ReviewName      = "slate-review"
	WritingName     = "slate-card-writing"
	ManagementName  = "slate-management"

	// ExtensionID 是 SEP-2640 定义的扩展标识。
	ExtensionID = "io.modelcontextprotocol/skills"
	// URIPrefix 和 URISuffix 是当前单文件 Skill 的规范资源地址组成部分。
	URIPrefix = "skill://"
	URISuffix = "/SKILL.md"
)

// CardWritingInstructions 同时用于旧客户端初始化和按需的卡片写作 Skill。
const CardWritingInstructions = `

用 Markdown 写卡片描述和评论。短段落或列表，**加粗重点与待决事项**。

1. 短句，一句一事。规则编号。删开篇多余依据、套话、常识和重复。
2. 需求先写用户何时能做什么、做完看到什么。
3. 限量时写谁来计数、怎么算、到上限会怎样。
4. 复用时写明对象和接法。字段、接口和验收写到可实施。
5. Git 提交哈希只写前 6 位。`

// Document 是单文件 Agent Skill。首版只接受 name、description 和 Markdown 正文，
// 由服务生成 frontmatter，避免资源中的 YAML 与目录声明不一致。
type Document struct {
	Name        string
	Description string
	Body        string
}

// URI 返回 Skill 的规范入口和资源 URI。
func URI(name string) string { return URIPrefix + name + URISuffix }

func (d Document) URI() string { return URI(d.Name) }

// Markdown 返回 SKILL.md 全文。引号保证 true、包含冒号等文本仍为 YAML 字符串。
func (d Document) Markdown() string {
	return "---\nname: " + strconv.Quote(d.Name) + "\ndescription: " + strconv.Quote(d.Description) + "\n---\n\n" + d.Body
}

//go:embed catalog/slate-kanban.md
var coreBody string

// 旧下载入口仍然返回完整的 SKILL.md。新目录复用其中详细流程的原文，
// 仅将跨章节引用改成指向相应 Skill，不维护两套守候和审查细则。
func legacySection(start, end string) string {
	_, text, ok := strings.Cut(SlateKanban(), start)
	if !ok {
		panic("缺少内置 Skill 章节: " + start)
	}
	if end != "" {
		text, _, ok = strings.Cut(text, end)
		if !ok {
			panic("缺少内置 Skill 章节: " + end)
		}
	}
	return strings.TrimSpace(text)
}

var builtins = makeBuiltins()

func makeBuiltins() []Document {
	plan := legacySection("### 先把方案写到任务评论", "## 四、派研发")
	plan = strings.ReplaceAll(plan, "第六节", URI(CoreName)+" 的澄清规则")
	watch := legacySection("## 三、守候和领取", "### 先把方案写到任务评论")
	watch = strings.NewReplacer(
		"第五节", URI(ReviewName),
		"第四节", URI(DevelopmentName),
		"完整提示词由 `get_board` 的 `watch_prompt` 返回", "守候按本 skill 和 `get_board` 的 `watch_prompt` 执行",
	).Replace(watch)
	dev := legacySection("## 四、派研发：一个任务一个子 agent、一个 worktree", "## 五、派审查")
	review := legacySection("## 五、派审查：只验收，不改代码", "## 六、做不下去的时候")
	review = strings.ReplaceAll(review, "第八节", "下文的合并流程")
	merge := legacySection("## 八、合并已审查的本地分支", "")
	merge = strings.ReplaceAll(merge, "第四节", URI(DevelopmentName))
	search := legacySection("列表查询：", "## 三、守候和领取")
	management := legacySection("只有用户明确要求管理看板设置、列或自动化时，才调用", "## 二、找到看板和项目路径")
	return []Document{
		{
			Name:        CoreName,
			Description: "Slate 看板调度入口：定位项目、核对身份与列指令、领取任务、并发名额、独立 worktree、方案确认及研发审查交接。处理 Slate 任务时使用；详细流程按场景加载。",
			Body:        coreBody,
		},
		{
			Name:        WatchName,
			Description: "长期守候 Slate 看板、监听事件、一次领满空闲名额、游标与断线恢复、Python 监听器及宿主超时配置。启动、恢复或排查守候时加载。",
			Body: "# 守候和领取\n\n先读取 `" + URI(CoreName) + "`，核对看板与身份。\n\n" + watch +
				"\n\nPython 监听器：读取 `" + WatchURI + "` 保存为 slate-watch.py；" +
				"运行 `python slate-watch.py --url \"<Slate MCP 地址>\" --board <看板ID> --name \"<whoami 的名称>\" --project \"<项目目录>\"`，" +
				"可用 `--host` 指定 codex、cursor、claude、opencode、pi、on-my-pi，自定义客户端用 `--command-json`。重启沿用同一 name。\n",
		},
		{
			Name:        DevelopmentName,
			Description: "收到研发任务、准备派发、建独立 worktree、确定研发方案、续租、自测提交、交接或返工时加载。",
			Body: "# 派发研发\n\n先按 `" + URI(CoreName) + "` 核对身份、路径和列规则。\n\n" +
				"## 方案确认\n\n" + plan + "\n\n" + dev + "\n",
		},
		{
			Name:        ReviewName,
			Description: "审查任务的验收证据、页面截图、逐项结论、返工以及适用的本机专属 Slate 分支合并、记录和清理。准备审查时加载。",
			Body: "# 审查和合并\n\n先按 `" + URI(CoreName) + "` 核对列规则和任务；附图用 `" + URI(WritingName) + "`。\n\n" +
				"## 方案确认\n\n" + plan + "\n\n## 审查\n\n" + review + "\n\n## 合并已审查的本地分支\n\n" + merge + "\n",
		},
		{
			Name:        WritingName,
			Description: "编写 Slate 卡片描述、评论与验收清单、查询任务列表语法和读取上传图片时加载。",
			Body: "# 卡片写作" + CardWritingInstructions + "\n\n## 查询任务\n\n列表查询：" + search +
				"\n\n## 图片\n\n" + legacySection("## 七、图片", "## 八、合并已审查的本地分支") + "\n",
		},
		{
			Name:        ManagementName,
			Description: "用户明确要求新建项目、管理看板设置、列或自动化时加载；仅执行用户授权的管理操作。",
			Body: "# 看板管理\n\n管理看板设置、列和自动化时，先用 `get_board` 读取当前配置。删除前请用户确认。\n\n" +
				"只有用户明确要求管理看板设置、列或自动化时，才调用 " + management + "\n\n" +
				"用户明确要求新建项目时用 `create_board`（slug 不填自动生成，template 默认研发，locale 按用户语言）；按 `" + URI(CoreName) +
				"` 登记已确认的项目路径。不擅自新建或修改已有路径。项目删除只能由人在网页上完成。\n",
		},
	}
}

// Builtins 返回内置 Skill 的独立列表，调用者不能修改编译进服务的内容。
func Builtins() []Document {
	return slices.Clone(builtins)
}

// Builtin 返回指定的只读内置 Skill。
func Builtin(name string) (Document, bool) {
	for _, doc := range builtins {
		if doc.Name == name {
			return doc, true
		}
	}
	return Document{}, false
}
