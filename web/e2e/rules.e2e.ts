// 自动化规则：卡片拖进某一列时，规则自动改卡片；停用的规则不触发。

import { expect, test } from '@playwright/test'
import { columnId, createBoard, createCard, createRule, getCard, uniqueSlug } from './api'
import { columnOf, dragTo, tileIn } from './board'

test('规则触发：卡片拖进进行中，自动加标签并以系统身份留言', async ({ page, request }) => {
  const board = await createBoard(request, `Rule ${uniqueSlug()}`)
  await createRule(request, board.id, {
    name: 'Mark started',
    trigger: 'card_entered',
    column_id: columnId(board, 'In progress'),
    actions: [
      { type: 'add_tag', tag: 'started' },
      { type: 'add_comment', text: 'Picked up automatically' },
    ],
  })
  const card = await createCard(request, board, 'To do', 'Plan the sprint')
  await page.goto(`/b/${board.id}`)

  const doing = columnOf(page, 'In progress')
  await dragTo(page, tileIn(columnOf(page, 'To do'), 'Plan the sprint'), doing)

  // 规则的效果直接出现在磁贴上，服务端也有记录
  await expect(tileIn(doing, 'Plan the sprint').getByText('started', { exact: true })).toBeVisible()
  await expect.poll(async () => (await getCard(request, card.id)).tags).toEqual(['started'])

  // 打开卡片：自动留的言以「System」的身份出现。刷新后再点，避开松手后 50 毫秒里 dnd-kit 吞掉点击
  await page.reload()
  await tileIn(columnOf(page, 'In progress'), 'Plan the sprint').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('Picked up automatically')).toBeVisible()
  await expect(dialog.getByText('System', { exact: true })).toBeVisible()
})

test('停用的规则不触发：卡片拖进去以后没有标签', async ({ page, request }) => {
  const board = await createBoard(request, `Off ${uniqueSlug()}`)
  await createRule(request, board.id, {
    name: 'Never runs',
    enabled: false,
    trigger: 'card_entered',
    column_id: columnId(board, 'In progress'),
    actions: [{ type: 'add_tag', tag: 'started' }],
  })
  const card = await createCard(request, board, 'To do', 'Plan the sprint')
  await page.goto(`/b/${board.id}`)

  const doing = columnOf(page, 'In progress')
  await dragTo(page, tileIn(columnOf(page, 'To do'), 'Plan the sprint'), doing)

  await expect(tileIn(doing, 'Plan the sprint')).toBeVisible()
  await expect.poll(async () => (await getCard(request, card.id)).column_id).toBe(columnId(board, 'In progress'))
  expect((await getCard(request, card.id)).tags).toEqual([])
  await expect(tileIn(doing, 'Plan the sprint').getByText('started', { exact: true })).toHaveCount(0)
})
