---
name: slate-kanban
description: 接入 Slate 看板替人干活的工作流程：确认看板和项目路径，守候看板，领取「进行中」列里的卡片，用子 agent 在独立的 git worktree 和分支上开发，做完交接到审查。用户提到 Slate、看板、领卡片、接任务、守候看板、处理进行中的卡片、按看板上的任务干活时使用。
---

# Slate 看板协作

你和人共用一个看板来管理开发工作。人把卡片放进「进行中」这类领取列，来安排你做什么；你做完把卡片交接到下一列（通常是「审查」），由人验收。

下面几条规则不能违反（服务端也会拦住，但你一开始就不要去试）：

- 只处理领取列里的卡片：`get_board` 返回的列里 `agent_pickup` 为 true 的那些，模板里叫「进行中」。别的列是人在规划。
- 做完用 `finish_card` 交接，不要用 `move_card` 把卡片拖进完成列：是否算完成，由人在审查之后决定。
- 优先级、依赖、到期时间、「只能由人处理」是人的安排，你不能改。工作中发现了新任务，用 `create_card` 记在领取列以外的列（例如「想法池」「待办」），不要放进领取列。
- 代码只在这张卡片自己的 git worktree 里改，不动项目目录本身，不动别的 worktree。
- 项目在本机的位置记在看板上（`project_path`），不要另外存在本地文件里。

## 一、先确认能用

调用 `whoami`。返回了你的名称，类型是 `agent`，说明 MCP 已经接好。调用不了，就请用户先安装：让用户把 Slate 服务地址下的 `/install.md`（例如 `http://localhost:7070/install.md`）交给你，按里面的步骤装 MCP。

这个 skill 的最新版可以从服务取：`/skills/slate-kanban/SKILL.md`，或者读 MCP 资源 `slate://skills/slate-kanban`。

## 二、找到看板和项目路径

1. 用 `list_boards` 列出看板。用户指定了看板就用它；否则找 `project_path` 等于当前 git 仓库根目录的那个（用 `git rev-parse --show-toplevel` 取根目录，比较前先都换成真实路径）。有多个匹配，或一个都没有，就列给用户选。
2. 用 `get_board` 读一遍每一列：`description` 是给人看的说明，`instruction` 是人写给你的做事规则，必须遵守。记下领取列的名字，以及 `next_column_id`（交接后卡片去哪一列）。
3. 看这个看板的 `project_path`：
   - 已经有值：直接用，不要再问用户。如果它和当前目录不是同一个仓库，先告诉用户并确认要在哪里工作；不要尝试修改它（你改不了，会得到 `ErrProjectPathLocked`）。
   - 是空的：问用户一次"这个看板对应的项目是 <当前仓库根目录> 吗"。用户确认后用 `set_project_path` 写下来；用户不确认，就先不开工。

## 三、守候和领取

你（主 agent）只做调度，不亲自写代码。同时进行的卡片数上限默认是 2，用户另有要求时按用户的。

```text
循环：
  1. 空闲名额 = 上限 - 正在做的卡片数
  2. 空闲名额大于 0 时领卡片：
     - 上限是 1：card = next_card(board_id)，card 为 null 表示没有可做的
     - 上限大于 1：对 list_cards(board_id, query='col:"<领取列名>" is:free -is:waiting -is:human')
       的前"空闲名额"张逐张 claim_card(card_id)
     领到的卡片都按第四节派发
  3. 刚派发了卡片，回到 1；否则守候：
     result = wait_for_updates(board_id, since=上一次的 cursor, timeout_seconds=30)
     记下 result 里的 cursor；timed_out 为 true 只是超时，不是错误
     first 或 truncated 为 true：用 list_cards 重新了解现状
     处理这批事件（见下面），然后回到 1
```

- 重启之后，先找回自己还持有的卡片：`list_cards(board_id, query="claimed:<你的名称>")`（名称来自 `whoami`），接着做它们。`next_card` 也会优先给你已经持有的卡片。
- 你离线期间发生的更新，上线后第一次 `wait_for_updates` 或 `pull_updates` 就会补给你，不用自己记时间。处理完这一批，才把返回的 `cursor` 当作下一次的 `since`。
- 事件里有 `type` 和 `card_id`。涉及你正在做的卡片时：
  - `comment.added`、`card.updated`：人有新的补充。用 `get_card` 读一下，转告子 agent。
  - `card.moved`、`card.released`：领取已经结束（被人收回或移走）。让子 agent 停下，把还没提交的成果告诉用户，不要继续改。
- 订阅资源 `slate://boards/{board_id}` 可以在看板变化时收到通知，但通知只是提示，可能丢失，不能代替上面的循环。

怎么守候，要看你所在的 host 允许什么：

| 情况 | 做法 |
|------|------|
| 工具调用可以挂一分钟左右（多数情况） | 用上面的阻塞循环，`timeout_seconds` 用 30，最大 50 |
| 工具调用的超时很短 | 把 `timeout_seconds` 调小（例如 10）；还是超时，就用下一行的办法 |
| host 不能一直保持会话，但有定时或 loop 功能 | 让 host 每隔几分钟执行一轮：领卡片，没有就结束这一轮，有就派发 |

## 四、派发：一张卡片一个子 agent、一个 worktree

1. 读卡片全文和全部评论（`next_card`、`claim_card` 已经返回了，不用重复读）。评论里人的补充和审查意见，比最初的描述优先；有不清楚的地方，用 `comment_card` 向人提问，不要猜。所在列的 `column_instruction` 必须遵守。
2. 在项目目录里建 worktree 和分支。名字里用卡片的 key（例如 SLATE-12）：

   ```sh
   git -C <project_path> worktree add <项目的上级目录>/<项目目录名>-<key 小写> -b slate/<key 小写>-<简短英文标题> <基础分支>
   ```

   基础分支：列的指令里指定了就用它，否则用项目的默认分支。分支或目录已经存在，说明这张卡片做过一半（返工，或者你重启过），直接进那个 worktree 接着做，不要重建。
   建好之后，用 `update_card` 把分支名写到卡片的 `branch` 上，让人在看板上就能看到这张卡片的代码在哪个分支。
3. 派一个子 agent 去做。告诉它：卡片全文和评论、`column_instruction`、worktree 的绝对路径，并写明：
   - 只在这个 worktree 里改动；
   - 按项目的惯例写代码、跑测试，通过之后提交到这个分支；
   - 不合并到主分支，不推送，除非列的指令明确要求；
   - 做完汇报：做了什么、怎么验证的、有什么风险、提交了什么。

   host 没有子 agent 的能力时，由你自己在这个 worktree 里做，一次只做一张。
4. 子 agent 工作期间，你至少每 20 分钟用 `comment_card` 在卡片下汇报一次进度：持有者评论会顺延领取的租约（默认 30 分钟）。预计要做很久的卡片，领取时用 `lease_minutes`（最多 1440）。
5. 子 agent 返回后，你先看它的汇报和分支上的提交，再调用 `finish_card`：`summary` 写清做了什么、怎么验证的、有什么风险；`branch` 填分支名；开了合并请求就填 `url`。它会把卡片移到下一列，并结束你的领取。
6. 人把卡片放回领取列并留了评论，说明要返工：你会再次领到它。读完新评论，回到原来的 worktree 接着改，再次 `finish_card`。

## 五、做不下去的时候

缺信息，或者需要人来决定：用 `comment_card` 把问题写清楚，再用 `release_card` 放手，不要占着卡片。

被服务端拒绝，说明做法不对，换一种做法，不要原样重试：

| 错误 | 含义 | 怎么办 |
|------|------|--------|
| `ErrNotHolder` | 这张卡片你已经不持有了（租约到期被别人接手，或被人收回） | 停止在这张卡片上的工作，先把没提交的成果告诉用户 |
| `ErrCardVersionConflict` | 你读完之后，卡片被别人改过 | 用 `get_card` 重新读，再决定怎么改 |
| `ErrAgentPickupColumn` | 想把卡片放进领取列 | agent 不能这样做，交接用 `finish_card` |
| `ErrColumnNotPickup` | 想领取的卡片不在领取列里 | 只处理领取列里的卡片 |
| `ErrProjectPathLocked` | 看板已经有项目路径 | 不要改，有问题告诉用户 |
| `ErrCardClaimed` | 别的 agent 正在做这张卡片 | 先做别的 |
| `ErrCardWaiting` | 它依赖的卡片还没完成 | 先做别的 |
| `ErrCardHumanOnly` | 这张卡片只能由人处理 | 跳过 |
| `ErrServiceUnavailable` | 服务正在关闭或重启 | 过几秒再试 |

## 六、收尾

人把卡片放进完成列、分支也已经合并之后，worktree 就没用了：执行 `git -C <project_path> worktree remove <路径>`。分支先用 `git branch --merged` 确认已经合并再删，没合并的不要删。
