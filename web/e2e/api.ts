// 用例的准备数据：直接调 REST 接口建项目、卡片和规则，省得每个用例都从界面一步步点。
// 界面上要验证的那一步（比如新建项目）自己走界面，不用这里的函数。

import { randomBytes } from 'node:crypto'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import type { BoardDetail, Card, CardView, Rule, RuleAction } from '../src/lib/api/types'

/** 项目模板：dev 五列，simple 三列（未着手、进行中、完成），empty 没有列 */
export type Template = 'dev' | 'simple' | 'empty'

/** 一个不会和别的用例撞车的卡片编号前缀：字母开头，共 9 位大写字母和数字 */
export function uniqueSlug(): string {
  return `E${randomBytes(4).toString('hex').toUpperCase()}`
}

/** 读出接口返回的 JSON；状态码不是 2xx 就抛错，并带上服务端的错误说明 */
async function json<T>(response: APIResponse): Promise<T> {
  if (!response.ok()) {
    throw new Error(`${response.url()} 返回 ${response.status()}：${await response.text()}`)
  }
  return (await response.json()) as T
}

/** 新建一个项目（默认三列的 simple 模板，一屏就能放下所有列，方便拖动） */
export async function createBoard(request: APIRequestContext, name: string, template: Template = 'simple'): Promise<BoardDetail> {
  return json<BoardDetail>(await request.post('/api/v1/boards', { data: { name, slug: uniqueSlug(), template, locale: 'en' } }))
}

/** 按列名找列的 id；找不到说明模板或语言变了，直接报错 */
export function columnId(board: BoardDetail, name: string): number {
  const column = board.columns.find((c) => c.name === name)
  if (!column) throw new Error(`项目里没有叫 ${name} 的列，现有的列：${board.columns.map((c) => c.name).join('、')}`)
  return column.id
}

/** 卡片的可选属性 */
export interface CardExtras {
  tags?: string[]
  priority?: number
  description?: string
}

/** 在指定列里新建一张卡片 */
export async function createCard(
  request: APIRequestContext,
  board: BoardDetail,
  columnName: string,
  title: string,
  extras: CardExtras = {},
): Promise<Card> {
  const data = { title, column_id: columnId(board, columnName), ...extras }
  return json<Card>(await request.post(`/api/v1/boards/${board.id}/cards`, { data }))
}

/** 新建一条自动化规则 */
export async function createRule(
  request: APIRequestContext,
  boardId: number,
  rule: { name: string; enabled?: boolean; trigger: Rule['trigger']; column_id: number; actions: RuleAction[] },
): Promise<Rule> {
  return json<Rule>(await request.post(`/api/v1/boards/${boardId}/rules`, { data: rule }))
}

/** 把卡片移到另一列的末尾 */
export async function moveCard(request: APIRequestContext, cardId: number, targetColumnId: number): Promise<Card> {
  return json<Card>(await request.post(`/api/v1/cards/${cardId}/move`, { data: { column_id: targetColumnId } }))
}

/** 读出卡片当前在服务端的样子 */
export async function getCard(request: APIRequestContext, cardId: number): Promise<CardView> {
  return json<CardView>(await request.get(`/api/v1/cards/${cardId}`))
}

/** 上传一张图片附件，返回能写进 Markdown 的地址 */
export async function uploadImage(request: APIRequestContext, board: BoardDetail, png: Buffer): Promise<string> {
  const res = await json<{ url: string }>(
    await request.post(`/api/v1/boards/${board.id}/attachments`, { multipart: { file: { name: 'cover.png', mimeType: 'image/png', buffer: png } } }),
  )
  return res.url
}
