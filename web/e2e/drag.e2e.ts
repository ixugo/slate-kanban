// 拖动卡片：跨列、同列换顺序、拖进收起的「完成」窄边；结果要当场可见，刷新后还在，服务端也记下了。

import { expect, test } from '@playwright/test'
import { columnId, createBoard, createCard, getCard, uniqueSlug } from './api'
import { columnOf, dragTo, railOf, tileIn } from './board'

test('拖动卡片到另一列：当场换列，刷新后还在，服务端也记下了', async ({ page, request }) => {
  const board = await createBoard(request, `Drag ${uniqueSlug()}`)
  const card = await createCard(request, board, 'To do', 'Ship the release')
  await page.goto(`/b/${board.id}`)

  const todo = columnOf(page, 'To do')
  const doing = columnOf(page, 'In progress')
  await dragTo(page, tileIn(todo, 'Ship the release'), doing)

  await expect(tileIn(doing, 'Ship the release')).toBeVisible()
  await expect(tileIn(todo, 'Ship the release')).toHaveCount(0)
  await expect.poll(async () => (await getCard(request, card.id)).column_id).toBe(columnId(board, 'In progress'))

  await page.reload()
  await expect(tileIn(columnOf(page, 'In progress'), 'Ship the release')).toBeVisible()
  await expect(tileIn(columnOf(page, 'To do'), 'Ship the release')).toHaveCount(0)
})

test('同一列里拖动换顺序：拖到最上面后刷新，顺序不变', async ({ page, request }) => {
  const board = await createBoard(request, `Order ${uniqueSlug()}`)
  for (const title of ['First card', 'Second card', 'Third card']) {
    await createCard(request, board, 'To do', title)
  }
  await page.goto(`/b/${board.id}`)

  const todo = columnOf(page, 'To do')
  const tiles = todo.getByRole('link')
  await expect(tiles).toHaveText([/First card/, /Second card/, /Third card/])

  // 把第三张拖到第一张的上半部分
  const first = tileIn(todo, 'First card')
  const box = await first.boundingBox()
  await dragTo(page, tileIn(todo, 'Third card'), first, -((box?.height ?? 40) / 4))
  await expect(tiles).toHaveText([/Third card/, /First card/, /Second card/])

  await page.reload()
  await expect(columnOf(page, 'To do').getByRole('link')).toHaveText([/Third card/, /First card/, /Second card/])
})

test('把卡片拖进收起的「完成」窄边：计数加一，展开后卡片在里面', async ({ page, request }) => {
  const board = await createBoard(request, `Done ${uniqueSlug()}`)
  const card = await createCard(request, board, 'In progress', 'Finish the report')
  await page.goto(`/b/${board.id}`)

  await expect(railOf(page, 'Done')).toHaveAccessibleName(/0 cards/)
  await dragTo(page, tileIn(columnOf(page, 'In progress'), 'Finish the report'), railOf(page, 'Done'))

  await expect(railOf(page, 'Done')).toHaveAccessibleName(/1 cards/)
  await expect(tileIn(columnOf(page, 'In progress'), 'Finish the report')).toHaveCount(0)
  await expect.poll(async () => (await getCard(request, card.id)).column_id).toBe(columnId(board, 'Done'))

  // 刷新后窄边上的计数还在；再点开，卡片就在「完成」列里。
  // 不在松手后立刻点：dnd-kit 会在松手后的 50 毫秒里吞掉点击，真人点不了这么快，但脚本可以
  await page.reload()
  await expect(railOf(page, 'Done')).toHaveAccessibleName(/1 cards/)
  await railOf(page, 'Done').click()
  await expect(tileIn(columnOf(page, 'Done'), 'Finish the report')).toBeVisible()
})
