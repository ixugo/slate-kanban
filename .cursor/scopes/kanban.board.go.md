# 看板与列（kanban 包 board.go / column.go / board_template.go / snapshot.go）

## 需求目标

不同项目的流程不一样：有的只要三列，有的要五列加评审。如果状态写死，用户没法按自己的习惯用；
但状态完全自由，AI 又不知道一张卡做完以后该去哪一列、哪些列里的卡该由它来做。
所以列由用户自己创建、改名、排序和删除，每一列带两段说明：给人看的说明，和给 agent 看的指令，
再加两个开关：这一列是否允许 agent 领活（AgentPickup）、卡片进入这一列是否算完成（IsDone），以及完成后默认去哪一列（NextColumnID）。
新建看板时按模板和界面语言生成一套起步的列，用户之后可以随意改。

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 不同项目需要不同数量、不同名字的列 | 固定状态会让用户削足适履，或者放弃使用 | 列完全由用户创建、改名、排序、删除；模板只提供起步的初始值 | 实施 |
| 2 | agent 不知道一列代表什么阶段，也不知道做完该移去哪里 | agent 乱移卡片或者停在原地不动 | 每列有 Description 给人看，Instruction 给 agent 看，NextColumnID 指明完成后的默认去向 | 实施 |
| 3 | 只有进行中的卡片才应该由 agent 处理 | agent 误动想法池、待办或评审中的卡片 | 列上有 AgentPickup 开关，领取、取下一张卡、完成卡片都只认开了开关的列；模板里只有进行中默认开启 | 实施 |
| 4 | 新建看板后一片空白，用户不知道怎么开始 | 用户要自己逐列创建，还得想每列该怎么写给 AI 的指令 | 提供 dev、simple、empty 三种模板，列名、说明和指令按界面语言生成，并同时创建默认的自动化规则 | 实施 |
| 5 | 看板标识要短、好记，并且不重复 | 卡片编号冲突，agent 引用错卡片 | slug 由 2 到 10 位大写字母和数字组成，全局唯一；不填时按名称生成，冲突自动加数字；手填冲突返回 409 | 实施 |
| 6 | 创建看板后又改 slug | 已发出的卡片编号全部失效，agent 记忆里的编号指向不存在的卡片 | slug 创建后不可修改 | 实施 |
| 7 | 删除一个还有卡片的列 | 卡片凭空消失，没有人知道去了哪里 | 列里有卡片时必须指定移入哪一列，否则返回 409；卡片按原有顺序追加到目标列末尾 | 实施 |
| 8 | 被删除的列还被其他列当作下一列，或被自动化规则引用 | 卡片完成时无处可去，规则执行时找不到列 | 删除时清除别的列对它的下一列引用；引用它的规则自动停用并记录事件，用户可以在规则页修正 | 实施 |
| 9 | 两个人同时调整列顺序 | 顺序错乱，甚至丢列 | 请求必须带完整的列 id 列表，和当前列集合不完全一致就返回 409；校验和重排在锁定看板行的事务里完成 | 实施 |
| 10 | agent 想新增、删除列或规则，或者删除看板 | 流程被 AI 改乱，人的设计被破坏 | 看板、列、规则的增删改只允许人操作；agent 唯一的例外是在项目路径为空时写入项目路径 | 实施 |
| 11 | 项目路径是某台电脑上的路径，多台电脑的路径不同 | 一个 agent 改写了路径，另一台电脑上的 agent 找不到目录 | 服务端只存一个主路径，agent 只能在它为空时写入；每台电脑自己的路径由 skill 保存在本机，见 skill 设计文档 | 实施 |
| 12 | 同一看板里两列同名 | 筛选和 agent 引用列名时有歧义 | 同一看板内列名不区分大小写唯一，重复返回 409 | 实施 |
| 13 | 单个看板列数没有上限 | 页面和 agent 的上下文被塞满 | 单个看板最多 30 列 | 实施 |
| 14 | 网页先加载整板，再建立实时连接，两者之间有空隙 | 空隙里的变化丢失，页面停留在旧状态 | 整板快照里带上读取数据之前的最新事件序号，网页从这个序号之后开始接收实时事件 | 实施 |
| 15 | 删除看板 | 卡片、评论、事件、游标、规则留下孤儿数据 | 在一个事务里按依赖顺序删除全部关联数据，最后广播一次唤醒等待者 | 实施 |

## 数据与限制

| 项 | 规则 |
|----|------|
| 看板名称 | 必填，去掉首尾空白后 1 到 100 个字符 |
| slug | 2 到 10 位，首字符是大写字母，其余是大写字母或数字；全局唯一；创建后不可改 |
| 看板说明 | 最多 2000 个字符 |
| 项目路径 | 最多 512 个字符；非空时必须是绝对路径（以斜线开头，或盘符加冒号加斜线或反斜线，或双反斜线开头）；不允许出现空字符 |
| 列名 | 必填，1 到 60 个字符，同一看板内不区分大小写唯一 |
| 列说明 | 最多 500 个字符 |
| agent 指令 | 最多 4000 个字符 |
| 每个看板的列数 | 最多 30 |
| 列间距 | Sort 以 1024 为间距，重排时整体重写 |

## 起步模板

模板 dev 是为开发工作准备的五列流程，simple 是三列，empty 不建任何列。列名、说明和指令按创建请求里的 locale 生成，支持 zh-CN、zh-TW、en、ja、ko，不认识的 locale 一律按 en 处理。

| 模板 | 列（按顺序） | AgentPickup | IsDone | 下一列 |
|------|-------------|-------------|--------|--------|
| dev | 想法池 | 否 | 否 | 待办 |
| dev | 待办 | 否 | 否 | 进行中 |
| dev | 进行中 | 是 | 否 | 审查 |
| dev | 审查 | 否 | 否 | 完成 |
| dev | 完成 | 否 | 是 | 无 |
| simple | 待办 | 否 | 否 | 进行中 |
| simple | 进行中 | 是 | 否 | 完成 |
| simple | 完成 | 否 | 是 | 无 |

dev 与 simple 同时创建三条默认的自动化规则，规则的细节见 kanban.automation.go.md：

| 规则 | 触发 | 动作 |
|------|------|------|
| 开始计时 | 卡片进入进行中 | 开始时间为空时写入当前时间 |
| 完成计时 | 卡片进入完成 | 完成时间为空时写入当前时间 |
| 重新打开 | 卡片离开完成 | 清空完成时间 |

## 创建看板

```mermaid
flowchart TD
    A[收到创建看板请求] --> B{操作者是人}
    B -->|否| Z1[返回 403, agent 不能创建看板]
    B -->|是| C[校验名称, 说明, 项目路径, 模板, locale]
    C -->|不通过| Z2[返回 400 并说明哪一项不合法]
    C -->|通过| D{请求带了 slug}
    D -->|是| E[转大写并校验格式]
    D -->|否| F[由名称生成 slug]
    E -->|格式不对| Z2
    E -->|通过| G[开启事务]
    F --> G
    G --> H{slug 已被使用}
    H -->|是, 手填| Z3[回滚, 返回 409]
    H -->|是, 自动生成| I[加数字后缀后重试, 最多 20 次]
    I --> H
    H -->|否| J[创建看板]
    J --> K[按模板创建各列]
    K --> L[回填各列的下一列]
    L --> M[按模板创建默认规则]
    M --> N[追加 board.created 事件]
    N --> O[提交事务并广播]
    O --> P[返回看板, 列和规则]
```

自动生成 slug 的办法：取名称里的 ASCII 字母和数字，转大写，最多取 6 位；结果不足 2 位或不是以字母开头（比如纯中文名）时，用 BOARD；
如果被占用，在末尾追加 2、3、4……，必要时截短前面的部分，保证总长度不超过 10 位。

## 修改看板

```mermaid
flowchart TD
    A[收到修改看板请求] --> B[读取看板, 不存在返回 404]
    B --> C{操作者是人}
    C -->|是| D[校验传入的字段]
    C -->|否| E{只改了项目路径, 且当前路径为空}
    E -->|否| Z1[返回 403]
    E -->|是| D
    D -->|不通过| Z2[返回 400]
    D -->|通过| F[开启事务, 锁定看板行]
    F --> G[只更新请求里出现的字段]
    G --> H{有字段真的变化}
    H -->|否| I[直接返回当前看板, 不写事件]
    H -->|是| J[追加 board.updated 事件, 提交并广播]
```

说明：判断 agent 能否写项目路径要在锁定看板行之后再读一次当前值，防止两个 agent 同时写入时后者覆盖前者。

## 新增与修改列

```mermaid
flowchart TD
    A[收到新增或修改列请求] --> B{操作者是人}
    B -->|否| Z1[返回 403]
    B -->|是| C[校验名称, 说明, 指令的长度]
    C -->|不通过| Z2[返回 400]
    C -->|通过| D[开启事务, 锁定看板行]
    D --> E{新增}
    E -->|是| F{该看板已有 30 列}
    F -->|是| Z3[回滚, 返回 400]
    F -->|否| G[Sort 取现有最大值加 1024]
    E -->|否| G2[读取列, 不存在返回 404]
    G --> H{名称在同一看板内不区分大小写重复}
    G2 --> H
    H -->|是| Z4[回滚, 返回 409]
    H -->|否| I{填了下一列}
    I -->|是| J{下一列属于同一看板且不是它自己}
    J -->|否| Z2
    J -->|是| K[保存列]
    I -->|否| K
    K --> L{有字段真的变化}
    L -->|否| M[直接返回当前列, 不写事件]
    L -->|是| N[追加 column.created 或 column.updated 事件, 提交并广播]
```

说明：修改时只更新请求里出现的字段；下一列填 0 表示清除。新增列时下一列只能指向已有的列，所以起步模板里的各列要先全部创建，再回填下一列。

## 删除列

```mermaid
flowchart TD
    A[收到删除列请求] --> B{操作者是人}
    B -->|否| Z1[返回 403]
    B -->|是| C[开启事务, 锁定看板行]
    C --> D[读取列, 不存在则按幂等返回成功]
    D --> E{列里有卡片}
    E -->|否| H
    E -->|是| F{请求带了移入目标列}
    F -->|否| Z2[回滚, 返回 409]
    F -->|是| G{目标列存在于同一看板且不是它自己}
    G -->|否| Z3[回滚, 返回 400]
    G -->|是| G2[把卡片按原顺序追加到目标列末尾]
    G2 --> H[清除别的列指向它的下一列引用]
    H --> I[停用引用它的规则并各追加一条规则更新事件]
    I --> J[删除列]
    J --> K[追加 column.deleted 事件]
    K --> L[提交并广播]
```

说明：

- 因删除列而被迁移的卡片不会触发自动化规则，因为这是结构调整，不是用户在流程里推进卡片；事件里只记录移动的张数，不为每张卡片单独写 card.moved。
- 卡片换了列，原来的领取不再有意义，所以被迁移的卡片领取一并清掉（有领取的写 card.released，why 为 moved），版本各加一。
- 列已经不存在时直接返回成功；移入目标列不存在、不属于该看板或就是要删的列本身，返回 400。
- 引用这一列的规则指触发所在的列是它，或者某个 move_to_column 动作的目标列是它；已经停用的规则不用动，想重新启用时保存前的校验会指出列不存在。

## 调整列顺序

```mermaid
flowchart TD
    A[收到列顺序请求, 带完整 id 列表] --> B{操作者是人}
    B -->|否| Z1[返回 403]
    B -->|是| C[开启事务, 锁定看板行]
    C --> D[读取该看板当前全部列]
    D --> E{列表与当前列集合完全一致且无重复}
    E -->|否| Z2[回滚, 返回 409]
    E -->|是| F[按列表顺序把 Sort 重写为 1024 的整数倍]
    F --> G[追加 column.reordered 事件]
    G --> H[提交并广播]
```

## 删除看板

```mermaid
flowchart TD
    A[收到删除看板请求] --> B{操作者是人}
    B -->|否| Z1[返回 403]
    B -->|是| C[开启事务, 锁定看板行]
    C --> D{看板存在}
    D -->|否| Z0[返回成功, 删除是幂等的]
    D -->|是| E[依次删除评论, 卡片, 规则, 列, agent 读取进度, 事件]
    E -->|任何一步失败| Z2[整体回滚, 返回数据库错误]
    E --> F[删除看板本身]
    F --> G[标记要广播, 但不写事件]
    G --> H[提交并广播一次]
```

说明：事件随看板一起删除，所以只广播、不写事件；正在等待这个看板的 agent 和网页被唤醒后重新查询，得到"看板不存在"。

入口：只有网页用的 REST `DELETE /boards/:board_id`，网页要求人在模态框里输入项目名称确认；MCP 两个视图都没有删除项目的工具（`mcpapi.TestNoBoardDeletionTool` 守住），agent 和管理视图都只能创建、修改项目。
删除顺序按依赖从子到父，每类数据一条删除语句，所以不管卡片多少，语句数是固定的。

## 整板快照

网页首次加载和断线恢复时使用。先读最新事件序号，再读数据，所以快照里的数据一定不比这个序号旧；
网页拿这个序号去接收实时事件，重复收到的事件只会触发一次多余的刷新，不会漏。

```mermaid
sequenceDiagram
    participant W as 网页
    participant C as Core
    participant D as 数据库

    W->>C: 读取看板快照
    C->>D: 查询该看板最新事件序号
    C->>D: 查询看板, 列
    C->>D: 查询全部卡片, 按列顺序和卡片位置排序
    C-->>W: 返回看板, 列, 规则, 卡片和 seq
    W->>C: 建立实时连接, Last-Event-ID 等于 seq
    C-->>W: 补发 seq 之后的事件, 然后转为实时
```

## 整板导出

网页工具栏的「导出」一次性拉出看板、列、规则、全部卡片（含列名、完成、等待、领取等派生字段）、每张卡的评论，以及导出时刻的最新事件序号。
不分页，规模按个人看板设计（几千张卡以内）。人和 agent 都能读，权限与整板快照相同；看板不存在返回 404。

| 格式 | 说明 |
|------|------|
| JSON（默认） | 顶层信封：`format: slate-board-export`、`version: 1`、`exported_at`（RFC3339）、`seq`、`board`、`columns`、`rules`、`cards[]`（每项含 `comments`） |
| Markdown | 在 REST 层渲染：`# 看板名（SLUG）`，按列 `##`，卡片 `### KEY 标题`，元信息行、描述、评论引用块；卡片顺序与看板一致 |

评论在 Core 里按看板批量读取，避免 N+1。REST 响应带 `Content-Disposition` 附件名和对应 `Content-Type`。

| 步骤 | 函数 | 文件 |
|------|------|------|
| 导出数据 | Core.ExportBoard | internal/core/kanban/export.go |
| 评论批量读 | CommentStorer.ListByBoard | internal/core/kanban/comment.go |
| JSON/Markdown 与路由 | API.export, renderBoardMarkdown | internal/web/api/kanbanapi/export.go, export_md.go |

## 权限小结

| 操作 | 人 | agent |
|------|----|-------|
| 创建、删除看板 | 允许 | 拒绝 |
| 修改看板名称、说明 | 允许 | 拒绝 |
| 写入项目路径 | 允许 | 仅当路径为空时允许 |
| 列的增删改与排序 | 允许 | 拒绝 |
| 读取看板、列、快照、导出 | 允许 | 允许 |

## 代码对照

| 步骤 | 函数 | 文件 |
|------|------|------|
| 看板存储接口与锁行 | BoardStorer, BoardStorer.Lock | internal/core/kanban/board.go |
| 创建、修改、删除、列表、详情 | Core.CreateBoard, createBoardOnce, UpdateBoard, DeleteBoard, ListBoards, GetBoard | internal/core/kanban/board.go |
| agent 能否写项目路径 | checkBoardPatchAllowed | internal/core/kanban/board.go |
| 删除看板的事务内步骤 | unit.deleteBoard | internal/core/kanban/board.go |
| slug 生成与校验 | generateSlug, validateSlug, maxSlugAttempts | internal/core/kanban/board.param.go |
| 项目路径校验 | validateProjectPath | internal/core/kanban/board.param.go |
| 起步模板的列与文案 | normalizeLocale, textsFor, unit.seedColumns | internal/core/kanban/board_template.go |
| 起步模板的规则 | defaultRules, unit.seedRules | internal/core/kanban/board_template.go |
| 列的增删改与排序 | Core.CreateColumn, UpdateColumn, DeleteColumn, ReorderColumns | internal/core/kanban/column.go |
| 删除列的事务内步骤 | unit.deleteColumn, evacuateColumn, disableRulesUsing, ruleUsesColumn | internal/core/kanban/column.go |
| 整板快照 | Core.BoardSnapshot | internal/core/kanban/snapshot.go |
| 整板导出 | Core.ExportBoard | internal/core/kanban/export.go |
| 入参结构 | CreateBoardInput, UpdateBoardInput, CreateColumnInput, UpdateColumnInput | internal/core/kanban/board.param.go, column.param.go |
| 错误定义 | ErrBoardNotFound, ErrColumnNotEmpty 等 | internal/core/kanban/errors.go |
