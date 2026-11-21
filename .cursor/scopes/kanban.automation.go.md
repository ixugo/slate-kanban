# 自动化规则（kanban 包 rule.go / rule_engine.go）

## 需求目标

Trello 里最受欢迎的能力之一是：任务移到某一列时，系统自动替你做一些事。本项目也需要，
最典型的两个：任务进入进行中，自动写开始时间；进入完成，自动写结束时间。
除此之外，用户还会有自己的习惯，比如进入审查时自动加标签，或者完成时自动评论。
所以做成可配置的规则：当什么事发生（触发）、满足什么条件（筛选语法）、做哪些事（动作）。
规则只能由人管理，在触发它的同一个事务里执行，执行结果会写进活动记录，让用户看得见系统为什么改了任务。

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 用户希望开始时间、完成时间自动记录 | 靠人手填，经常漏，统计不准 | 触发是任务进入或离开某列，动作是写时间；起步模板自带三条默认规则 | 实施 |
| 2 | 用户有各自的流程习惯，例如进入审查自动加标签 | 写死的逻辑满足不了，只能手工重复操作 | 规则由用户配置：触发（创建、进入列、离开列）、条件（沿用任务筛选语法）、动作（十二种） | 实施 |
| 3 | 规则之间互相触发，例如进入 A 就移到 B，进入 B 又移回 A | 请求卡死，数据库被无限写入 | 链式深度最多 3 层；同一条规则在同一条链里最多执行一次 | 实施 |
| 4 | 规则引用的列被删除 | 执行时找不到列而出错 | 删除列时停用引用它的规则，见看板设计 | 实施 |
| 5 | 规则配置写错，保存后才发现永远不触发 | 用户花大量时间排查 | 保存时校验触发类型、列归属、条件语法和每个动作的参数；执行时写 automation.fired 事件，活动页可见 | 实施 |
| 6 | 某个动作无法执行（例如标签已满 10 个） | 要么让用户的拖动失败，体验差；要么悄悄吞掉，没人知道 | 动作按能做就做的语义执行，参数问题导致无法执行的动作跳过，并在 automation.fired 的 skipped 里列出；只有数据库错误才回滚整个操作 | 实施 |
| 7 | 自动化改了任务，但看不出是谁改的 | 用户不明白任务为什么自己变了 | 自动化产生的事件与评论，操作者类型是 system，名字是 automation 加冒号加规则名 | 实施 |
| 8 | 人手动清除了开始时间，下次进入进行中又被覆盖 | 人的修正被自动覆盖 | 写时间的动作默认模式为 if_empty，只在为空时写入；选择 always 才覆盖 | 实施 |
| 9 | 规则数量和动作数量没有上限 | 每次变更都要遍历大量规则，拖慢请求 | 每个看板最多 50 条规则，每条规则最多 20 个动作；一次请求里规则列表只查询一次 | 实施 |
| 10 | agent 修改规则 | 人设计的自动化被 AI 改乱 | 规则的增删改只允许人，agent 只能读 | 实施 |
| 11 | agent 完成交接或人拖动时，规则是否生效 | 结果取决于谁操作，行为不一致 | 创建、移动、完成交接三个入口走同一个规则引擎，不区分操作者 | 实施 |
| 12 | 因删除列而被迁移的任务 | 规则大量触发，产生意料之外的写入 | 结构调整不触发规则，见看板设计 | 实施 |

## 规则的组成

| 部分 | 取值 | 说明 |
|------|------|------|
| 名称 | 1 到 100 个字符 | 出现在活动记录里 |
| 启用 | 是或否 | 停用的规则不执行 |
| 触发 | card_created、card_entered、card_left | 创建任务；任务进入某列；任务离开某列 |
| 列 | 0 或某列的 id | 对 card_entered 与 card_left 表示哪一列；0 表示任意列；对 card_created 表示创建在哪一列，0 表示任意 |
| 条件 | 筛选语法，可为空，最多 500 个字符 | 用触发之后的任务状态匹配，不满足就不执行 |
| 动作 | 1 到 20 个，按顺序执行 | 见下表 |

创建任务会同时触发 card_created 和进入初始列的 card_entered。

## 动作

| 类型 | 参数 | 效果 |
|------|------|------|
| set_started_at | mode：if_empty（默认）或 always | 开始时间设为当前时间 |
| set_finished_at | mode：if_empty（默认）或 always | 完成时间设为当前时间 |
| clear_started_at | 无 | 清空开始时间 |
| clear_finished_at | 无 | 清空完成时间 |
| set_due | days：1 到 365 | 到期时间设为当前时间加 days 乘 24 小时 |
| clear_due | 无 | 清空到期时间 |
| set_priority | priority：0 到 3 | 设置优先级 |
| add_tag | tag：1 到 30 个字符 | 加标签（不区分大小写）；已有则什么都不做；已满 10 个则跳过并记入 skipped |
| remove_tag | tag | 移除标签（不区分大小写）；没有则什么都不做 |
| set_human_only | flag：是或否 | 设置是否只能由人处理 |
| add_comment | text：1 到 2000 个字符 | 以 system 身份写一条评论 |
| move_to_column | column_id：同一看板的列 | 把任务移到目标列末尾，并继续触发规则；目标列已不存在则跳过并记入 skipped；任务已在目标列则什么都不做 |

一个动作执行后有三种结果：生效；本来就不需要改（例如时间已有值而模式是 if_empty、标签已存在），算空操作，不记录；
因为参数原因无法执行（例如标签已满、目标列已被删除），跳过并记入 skipped，格式是"动作类型：原因"。

## 起步规则

新看板按模板在创建事务里写入默认规则（进入进行中写开始时间、进入完成写完成时间、离开完成清空完成时间），
只在模板里有对应的列时才有。它们和用户自己建的规则完全一样，可以停用、修改、删除；
创建看板的 board.created 事件已经说明了看板的诞生，所以起步规则不单独产生 rule.created 事件。
用户删除后不会自动补回，因为规则是用户可管理的数据，不是内置逻辑。

## 执行流程

引擎在三个地方被调用：创建任务、移动任务、完成交接。调用时任务的新状态已经在内存里，但还没保存；
引擎修改内存里的任务，最后由调用方保存一次，所以一次操作里不管有多少条规则命中，任务只写一次、版本只加一次。

```mermaid
flowchart TD
    A[调用方传入任务, 触发类型, 列, 当前深度, 已执行规则集合] --> B{深度超过 3}
    B -->|是| Z1[停止, 记录警告日志]
    B -->|否| C[取该看板启用的规则, 一次事务里只查一次]
    C --> D[按顺序遍历规则]
    D --> E{触发类型和列匹配}
    E -->|否| D
    E -->|是| F{这条规则已在本条链里执行过}
    F -->|是| D
    F -->|否| G{条件为空或筛选命中}
    G -->|否| D
    G -->|是| H[把规则加入已执行集合]
    H --> I[按顺序执行每个动作]
    I --> J{动作是 move_to_column}
    J -->|否| K[修改内存里的任务, 参数无法执行则记入 skipped]
    J -->|是| L[改列, 追加到目标列末尾, 写 card.moved 事件]
    L --> M[用深度加一递归触发离开原列和进入目标列]
    K --> N{还有动作}
    M --> N
    N -->|是| I
    N -->|否| O{有生效的动作或被跳过的动作}
    O -->|是| O2[写 automation.fired 事件]
    O -->|否| D
    O2 --> D
    D -->|遍历完| P[返回, 由调用方保存任务]
```

说明：

- 条件匹配用的是触发之后的任务状态，所以可以写成：进入审查，并且带有 backend 标签，才执行。
- 一次操作指创建一个任务、移动一个任务或完成一次交接。同一次操作里，一条规则只要执行过一次，就加入集合，递归里不会再执行它，保证不会出现 A 到 B 再到 A 的无限循环。
- 深度从 0 开始：人的操作是第 0 层，规则把任务移走是第 1 层，超过 3 层不再触发规则，只记一条警告日志。
- move_to_column 递归产生的 card.moved 事件，操作者是 system，名字为 automation 加冒号加规则名。
- add_comment 需要任务已经有 id；创建任务时先保存再执行规则，所以评论能写入。
- 创建出来的任务只有被规则改过自身字段才会保存第二次，所以它会是第二版；规则一个字段都没改，它还是第一版。
- 规则不动领取：创建时本来没有；agent 换列和完成交接在进入引擎之前先清；人换列时领取保留。move_to_column 只管列和位置，不需要再清。
- 保存规则时条件已经校验过；运行时如果读到无法解析的条件（只可能是数据被改坏），记一条警告日志并忽略这条规则，不让用户的操作失败。
- 没有任何动作生效、也没有动作被跳过时不写 automation.fired，免得活动记录被没有意义的条目刷屏。
- automation.fired 事件的内容是任务编号、规则编号、规则名、生效的动作类型列表 actions，以及被跳过的动作列表 skipped。
- 保存规则时的静态校验：触发类型在白名单内；列属于该看板；条件能被筛选解析器解析；每个动作的类型在白名单内，参数范围正确；move_to_column 的目标列存在。

## 规则的管理

```mermaid
flowchart TD
    A[收到规则的创建, 修改或删除请求] --> B{操作者是人}
    B -->|否| Z1[返回 403]
    B -->|是| C[校验名称, 触发, 列, 条件, 动作]
    C -->|不通过| Z2[返回 400, 指出哪一项不合法]
    C -->|通过| D[开启事务, 锁定看板行]
    D --> E{新增}
    E -->|是| F{该看板已有 50 条规则}
    F -->|是| Z3[回滚, 返回 400]
    F -->|否| G[Sort 取最大值加 1024, 保存]
    E -->|否| H[读取规则, 不存在返回 404, 删除时按幂等返回成功]
    H --> I[应用修改或删除]
    G --> J[追加 rule.created, rule.updated 或 rule.deleted 事件]
    I --> J
    J --> K[提交并广播]
```

## 一个例子：拖进进行中

```mermaid
sequenceDiagram
    participant U as 人
    participant C as Core
    participant R as 规则引擎
    participant D as 数据库

    U->>C: 把 SLATE-12 拖到进行中
    C->>D: 开启事务, 锁定看板行
    C->>D: 读取任务, 计算新位置, 改列
    C->>D: 追加 card.moved 事件
    C->>R: 触发离开待办与进入进行中
    R->>D: 读取启用的规则, 只查一次
    R->>R: 命中开始计时规则, 开始时间为空则写入当前时间
    R->>D: 追加 automation.fired 事件
    R-->>C: 返回, 任务已被修改
    C->>D: 保存任务, Version 加一
    C->>D: 提交事务
    C-->>U: 返回最新的任务
```

## 代码对照

| 步骤 | 函数 | 文件 |
|------|------|------|
| 规则存储接口 | RuleStorer | internal/core/kanban/rule.go |
| 规则校验 | normalizeRule, normalizeActions, normalizeAction | internal/core/kanban/rule.go |
| 动作类型、触发类型、数量上限常量 | ActionSetStartedAt 等, TriggerCardEntered 等, maxRulesPerBoard, maxRuleActions | internal/core/kanban/rule.go |
| 入参结构 | CreateRuleInput, UpdateRuleInput | internal/core/kanban/rule.param.go |
| 规则的增删改查 | Core.ListRules, CreateRule, UpdateRule, DeleteRule | internal/core/kanban/rule_manage.go |
| 引擎的运行状态与深度上限 | ruleRun, newRuleRun, maxRuleDepth | internal/core/kanban/rule_engine.go |
| 创建任务时触发 | unit.fireCreateRules, ruleRun.created | internal/core/kanban/rule_engine.go |
| 换列和完成交接时触发 | ruleRun.moved, unit.commitMove | internal/core/kanban/rule_engine.go, card_move.go |
| 匹配并执行规则 | ruleRun.trigger, listensTo, conditionHolds, execute | internal/core/kanban/rule_engine.go |
| 单个动作 | ruleRun.apply, applyFieldAction | internal/core/kanban/rule_engine.go, rule_action.go |
| 移动到列的动作 | ruleRun.move, unit.appendPosition | internal/core/kanban/rule_engine.go, card_position.go |
| 模板里的默认规则 | defaultRules, unit.seedRules | internal/core/kanban/board_template.go |
| 规则的数据库实现 | Rule | internal/core/kanban/stores/kanbandb/rule.db.go |
