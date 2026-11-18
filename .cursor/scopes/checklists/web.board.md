# 看板页面需求质量清单

用途：`web.board.md` 的需求质量自检  
所有者：AI 生成，用户终审

## 完整性

- [x] CHK001 - 拖拽落点（同列、跨列、空列、列尾、取消、原地）在 `dnd.test.ts`、`handleDragEnd.test.ts` 有覆盖；手工路径未全量列出。[Gap 已缩小]
- [x] CHK002 - `web.board.md` 与实现已区分：短字段失焦/变更即 PATCH；描述与评论要点保存；标签走 `TagEditor` 即时提交。
- [x] CHK003 - v1 不做看板级活动流，文档已列入「v1 明确不做」。
- [x] CHK004 - 搜索无结果（`search.noResults`）、语法错误（`role="alert"`）、截断（`search.truncated`）均有 UI；`BoardPage.test.tsx` 覆盖语法错误。
- [ ] CHK005 - 看板列表空、整板无卡、列内无卡、无评论等空状态文案有零散实现，但未在 `web.board.md` 逐条验收表。[Gap：需在 `BoardListPage` / `Comments` 补文档或补测]

## 清晰度

- [x] CHK006 - `planMove` 与「原地放下」在 `dnd.test.ts` 有明确用例；与服务端一致以集成+流刷新为准。
- [x] CHK007 - `computePosition`/`applyMoveToSnapshot` 单测对齐 `POSITION_STEP=1024`；不一致由失败回滚与 SSE 重取兜底。
- [x] CHK008 - 版本冲突：toast + 失效详情 query；输入框保留（见 `CardDialog.test.tsx`「版本冲突时不丢输入框内容」）。

## 一致性

- [ ] CHK009 - 筛选说明在 `SearchHelp`，与服务端 `card_filter.go` 是否完全一致需后端对照。[未在本轮核对]
- [x] CHK010 - 看板页不展示活动事件载荷；卡片弹窗无事件时间线，不适用。
- [x] CHK011 - v1 无 `activity` 网页接口；与 `events` 一致性留待活动流功能。
- [ ] CHK012 - 「释放领取」「删除卡片」前端对人可见；删除评论权限与 agent 拒绝需对照 `api.rest.go.md` + Core。[未写 E2E]

## 边界场景

- [ ] CHK013 - 标题/标签/描述有 `limits` 常量与前端正则，超长布局未做专项视觉测。[Edge Case]
- [ ] CHK014 - 拖动中 SSE 刷新会重置 `items`，极端竞态未单测。[Edge Case]
- [x] CHK015 - `markdown.test.tsx` 覆盖 script、`javascript:`、HTML 图片语法；描述/评论共 `markdown.tsx`。
- [ ] CHK016 - 弹窗打开时卡被删/移列：404 显示 `card.notFound`；移列依赖 invalidate，未单独集成测。[Edge Case]
- [x] CHK017 - 键盘：`/` `n`、磁贴 Enter 打开/Space+方向键拖拽（`CardTile.test.tsx`）、**窄边 Enter 展开**（`BoardPage.test.tsx`）；弹窗 Esc 由 Radix。[部分：关闭弹窗专用测仍无]
- [x] CHK021 - 列收起：默认 `is_done`、localStorage 键、`useDroppable` 落点、`handleDragEnd` 列落点单测、无障碍文案 `board.collapseColumn` / `expandColumn`。
- [x] CHK022 - 松手后的落下动画：在真实浏览器里用 requestAnimationFrame 逐帧采样（跨列、同列各一次），松手后没有任何一帧卡片回到旧位置，浮起的卡片终点与真实卡片重合（偏差 < 0.5px），放大、倾斜、阴影收回到平放；纯函数部分见 `dnd.test.ts`（`pickShownItems`）、`dropAnimation.test.ts`。
- [x] CHK023 - 点选筛选：真实浏览器里点选标签、列、状态、优先级后，看板只剩命中的卡片，命中数 / 卡片总数显示正确；亮色、暗色和 390 像素宽的手机屏上弹层都不超出屏幕，弹层很高时在内部滚动；点选不会让工具栏上的标题和搜索框错位。
- [x] CHK024 - 点选拼出的筛选词与服务端 `|` 多选一语法一一对应（`card_filter_test.go` 与 `filters.test.ts` 用同样的写法），含引号或竖线的标签不会拼出错误条件。

## v1 未做（与产品卡对齐）

- [x] CHK019 - 列表视图（表格）— `web.list.md`；`sortCards.test.ts`、`ListView.test.tsx`、`BoardView.test.tsx`。
- [x] CHK020 - 磁贴中键/Cmd+点击在新标签页打开卡片，同时保持单一 Tab 停靠点（`CardTile.test.tsx`；真实 Chrome 里验证过中键、Cmd+点击新开 `?card=` 页，左键不整页刷新，拖拽松手不弹卡片）。