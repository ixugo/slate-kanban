// 每个 REST 接口一个函数，路径和方法对应 .cursor/scopes/api.rest.go.md 里的路由表。
// 这里只负责拼路径和入参，不做任何业务判断（权限和校验都在服务端）。

import { request, upload } from './client'
import type {
  ActivityPage,
  ActivityQuery,
  Attachment,
  AuthStatus,
  Board,
  BoardDetail,
  CardChange,
  CardDetail,
  CardList,
  Card,
  Column,
  Comment,
  ConnectGuide,
  CreateBoardInput,
  CreateCardInput,
  CreateColumnInput,
  CreateRuleInput,
  EventPage,
  MoveCardInput,
  Rule,
  Snapshot,
  UpdateBoardInput,
  UpdateCardInput,
  UpdateColumnInput,
  UpdateRuleInput,
  UpdateApplied,
  UpdateInfo,
  VersionInfo,
  ProjectTemplateSummary,
  ProjectTemplateDetail,
  WebhookView,
  CreateWebhookInput,
  UpdateWebhookInput,
  TestWebhookResult,
} from './types'

/** 取消请求用的信号，查询函数把 TanStack Query 给的 signal 传进来 */
type Signal = AbortSignal | undefined

/** 没有内容的接口（删除、释放）的返回值 */
export type Empty = Record<string, never>

/** 全部看板 */
export function listBoards(signal?: Signal): Promise<Board[]> {
  return request<Board[]>('/boards', { signal })
}

/** 创建看板，按模板和语言生成起步的列和规则 */
export function createBoard(input: CreateBoardInput): Promise<BoardDetail> {
  return request<BoardDetail>('/boards', { method: 'POST', body: input })
}

/** 修改看板的名称、说明、项目路径 */
export function updateBoard(boardId: number, input: UpdateBoardInput): Promise<Board> {
  return request<Board>(`/boards/${boardId}`, { method: 'PATCH', body: input })
}

/** 删除看板，连同它的列、卡片、规则 */
export function deleteBoard(boardId: number): Promise<Empty> {
  return request<Empty>(`/boards/${boardId}`, { method: 'DELETE' })
}

/** 整板快照：看板、列、规则、全部卡片和事件序号 */
export function getSnapshot(boardId: number, signal?: Signal): Promise<Snapshot> {
  return request<Snapshot>(`/boards/${boardId}/snapshot`, { signal })
}

/** 读取序号大于 after 的事件，按序号升序 */
export function listEvents(boardId: number, after: number, limit: number, signal?: Signal): Promise<EventPage> {
  return request<EventPage>(`/boards/${boardId}/events`, { query: { after, limit }, signal })
}

/** 读活动流：事件从新到旧，可按事件类型和操作者筛选，before 是翻页起点 */
export function listActivity(boardId: number, query: ActivityQuery, signal?: Signal): Promise<ActivityPage> {
  const { types, exclude_types, ...rest } = query
  return request<ActivityPage>(`/boards/${boardId}/activity`, {
    query: {
      ...rest,
      types: types && types.length > 0 ? types.join(',') : undefined,
      exclude_types: exclude_types && exclude_types.length > 0 ? exclude_types.join(',') : undefined,
    },
    signal,
  })
}

/** 在最右边新增一列 */
export function createColumn(boardId: number, input: CreateColumnInput): Promise<Column> {
  return request<Column>(`/boards/${boardId}/columns`, { method: 'POST', body: input })
}

/** 修改列，只提交出现的字段 */
export function updateColumn(boardId: number, columnId: number, input: UpdateColumnInput): Promise<Column> {
  return request<Column>(`/boards/${boardId}/columns/${columnId}`, { method: 'PATCH', body: input })
}

/** 按给定的顺序重排所有列，ids 必须正好是看板当前全部的列 */
export function reorderColumns(boardId: number, ids: number[]): Promise<Column[]> {
  return request<Column[]>(`/boards/${boardId}/columns/order`, { method: 'PUT', body: { ids } })
}

/** 删除列；列里有卡片时必须用 moveTo 指定搬去哪一列 */
export function deleteColumn(boardId: number, columnId: number, moveTo?: number): Promise<Empty> {
  return request<Empty>(`/boards/${boardId}/columns/${columnId}`, {
    method: 'DELETE',
    query: { move_to: moveTo },
  })
}

/** 按筛选语法查询卡片，query 为空返回全部（受 limit 限制） */
export function searchCards(boardId: number, query: string, limit: number, signal?: Signal): Promise<CardList> {
  return request<CardList>(`/boards/${boardId}/cards`, { query: { query, limit }, signal })
}

/** 创建卡片，column_id 不填放进第一列 */
export function createCard(boardId: number, input: CreateCardInput): Promise<Card> {
  return request<Card>(`/boards/${boardId}/cards`, { method: 'POST', body: input })
}

/** 一张卡片的完整信息：依赖、评论、所在列的 agent 指令 */
export function getCardDetail(cardId: number, signal?: Signal): Promise<CardDetail> {
  return request<CardDetail>(`/cards/${cardId}`, { signal })
}

/** 卡片的一次变更记录：修改前、修改后的完整内容，活动里的「查看变更」用它左右对比 */
export function getCardChange(cardId: number, changeId: number, signal?: Signal): Promise<CardChange> {
  return request<CardChange>(`/cards/${cardId}/changes/${changeId}`, { signal })
}

/** 修改卡片；带 version 时服务端会检查有没有被别人先改过 */
export function updateCard(cardId: number, input: UpdateCardInput): Promise<Card> {
  return request<Card>(`/cards/${cardId}`, { method: 'PATCH', body: input })
}

/** 删除卡片 */
export function deleteCard(cardId: number): Promise<Empty> {
  return request<Empty>(`/cards/${cardId}`, { method: 'DELETE' })
}

/** 移动卡片：换列、同列内调整顺序 */
export function moveCard(cardId: number, input: MoveCardInput): Promise<Card> {
  return request<Card>(`/cards/${cardId}/move`, { method: 'POST', body: input })
}

/** 把任务未读动态清零，打开详情时调用 */
export function markCardRead(cardId: number): Promise<Empty> {
  return request<Empty>(`/cards/${cardId}/read`, { method: 'POST' })
}

/** 收回 agent 对卡片的领取 */
export function releaseCard(cardId: number): Promise<Empty> {
  return request<Empty>(`/cards/${cardId}/release`, { method: 'POST' })
}

/** 给卡片加评论 */
export function addComment(cardId: number, body: string): Promise<Comment> {
  return request<Comment>(`/cards/${cardId}/comments`, { method: 'POST', body: { body } })
}

/** 删除评论 */
export function deleteComment(commentId: number): Promise<Empty> {
  return request<Empty>(`/comments/${commentId}`, { method: 'DELETE' })
}

/** 创建自动化规则 */
export function createRule(boardId: number, input: CreateRuleInput): Promise<Rule> {
  return request<Rule>(`/boards/${boardId}/rules`, { method: 'POST', body: input })
}

/** 修改自动化规则，只提交出现的字段 */
export function updateRule(ruleId: number, input: UpdateRuleInput): Promise<Rule> {
  return request<Rule>(`/rules/${ruleId}`, { method: 'PATCH', body: input })
}

/** 删除自动化规则 */
export function deleteRule(ruleId: number): Promise<Empty> {
  return request<Empty>(`/rules/${ruleId}`, { method: 'DELETE' })
}

/** 接入说明：MCP 地址、skill 地址，以及各 agent 宿主的安装命令（工作视图和管理视图各一条） */
export function getConnectGuide(signal?: Signal): Promise<ConnectGuide> {
  return request<ConnectGuide>('/connect', { signal })
}

/** 检查有没有新版本（服务端去问 GitHub，查询失败时按没有更新返回） */
export function getUpdateInfo(signal?: Signal): Promise<UpdateInfo> {
  return request<UpdateInfo>('/update', { signal })
}

/** 立即更新：服务端下载、校验并替换本地程序，返回新版本号 */
export function applyUpdate(): Promise<UpdateApplied> {
  return request<UpdateApplied>('/update', { method: 'POST' })
}

/** 服务端的构建版本，「关于」弹窗里显示 */
export function getVersion(signal?: Signal): Promise<VersionInfo> {
  return request<VersionInfo>('/version', { signal })
}

/** 访问口令状态：服务端是否启用、当前浏览器是否已通过；这个接口本身不要求口令 */
export function getAuthStatus(signal?: Signal): Promise<AuthStatus> {
  return request<AuthStatus>('/auth/status', { signal })
}

/** 用访问口令登录，成功后服务端会写入会话 cookie */
export function login(passcode: string): Promise<{ authenticated: boolean }> {
  return request<{ authenticated: boolean }>('/auth/login', { method: 'POST', body: { passcode } })
}

/** 退出登录，清掉会话 cookie */
export function logout(): Promise<Empty> {
  return request<Empty>('/auth/logout', { method: 'POST' })
}

/** 把粘贴的图片上传到看板，返回可写进 Markdown 的地址 */
export function uploadAttachment(boardId: number, file: Blob): Promise<Attachment> {
  return upload<Attachment>(`/boards/${boardId}/attachments`, 'file', file)
}

/** 全部自定义项目模板 */
export function listProjectTemplates(signal?: Signal): Promise<ProjectTemplateSummary[]> {
  return request<ProjectTemplateSummary[]>('/templates', { signal })
}

/** 导入项目模板 JSON；replace 为真时覆盖同名 */
export function importProjectTemplate(body: unknown, replace?: boolean): Promise<ProjectTemplateDetail> {
  return request<ProjectTemplateDetail>('/templates', {
    method: 'POST',
    body,
    query: replace ? { replace: true } : undefined,
  })
}

/** 删除自定义项目模板 */
export function deleteProjectTemplate(templateId: number): Promise<Empty> {
  return request<Empty>(`/templates/${templateId}`, { method: 'DELETE' })
}

/** 看板的活动 webhook 列表 */
export function listWebhooks(boardId: number, signal?: Signal): Promise<WebhookView[]> {
  return request<WebhookView[]>(`/boards/${boardId}/webhooks`, { signal })
}

/** 新增活动 webhook */
export function createWebhook(boardId: number, input: CreateWebhookInput): Promise<WebhookView> {
  return request<WebhookView>(`/boards/${boardId}/webhooks`, { method: 'POST', body: input })
}

/** 修改活动 webhook */
export function updateWebhook(webhookId: number, input: UpdateWebhookInput): Promise<WebhookView> {
  return request<WebhookView>(`/webhooks/${webhookId}`, { method: 'PATCH', body: input })
}

/** 删除活动 webhook */
export function deleteWebhook(webhookId: number): Promise<Empty> {
  return request<Empty>(`/webhooks/${webhookId}`, { method: 'DELETE' })
}

/** 测试发送 webhook */
export function testWebhook(webhookId: number): Promise<TestWebhookResult> {
  return request<TestWebhookResult>(`/webhooks/${webhookId}/test`, { method: 'POST' })
}
