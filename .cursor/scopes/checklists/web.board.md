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
- [x] CHK017 - 键盘：`/` `n`、磁贴 Enter/Space+方向键、**窄边 Enter 展开**（`BoardPage.test.tsx`）；弹窗 Esc 由 Radix。[部分：关闭弹窗专用测仍无]
- [x] CHK021 - 列收起：默认 `is_done`、localStorage 键、`useDroppable` 落点、`handleDragEnd` 列落点单测、无障碍文案 `board.collapseColumn` / `expandColumn`。

## v1 未做（与产品卡对齐）

- [ ] CHK019 - 列表视图（表格）— **未做**，Slate 看板 **SLATE-38**。
- [ ] CHK020 - 磁贴中新开标签页打开卡片 — **未做**（单 Tab 与 `?card=`），Slate 看板 **SLATE-35**。