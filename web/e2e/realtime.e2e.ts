// 实时同步：两个窗口打开同一个项目，一边的改动不用刷新就出现在另一边。

import { expect, test } from '@playwright/test'
import { columnId, createBoard, createCard, moveCard, uniqueSlug } from './api'
import { columnOf, tileIn } from './board'

test('一个窗口里新建卡片，另一个窗口不刷新就能看到；外部的移动两边都跟着变', async ({ context, request }) => {
  const board = await createBoard(request, `Live ${uniqueSlug()}`)
  const a = await context.newPage()
  const b = await context.newPage()
  await a.goto(`/b/${board.id}`)
  await b.goto(`/b/${board.id}`)
  await expect(columnOf(b, 'To do')).toBeVisible()

  // 窗口 A 在「To do」里新建一张卡片，窗口 B 自己出现它
  await columnOf(a, 'To do').getByRole('button', { name: 'Add a card' }).click()
  await columnOf(a, 'To do').getByRole('textbox', { name: 'Add a card' }).fill('Written in window A')
  await columnOf(a, 'To do').getByRole('textbox', { name: 'Add a card' }).press('Enter')
  await expect(tileIn(columnOf(a, 'To do'), 'Written in window A')).toBeVisible()
  await expect(tileIn(columnOf(b, 'To do'), 'Written in window A')).toBeVisible()

  // 另一个来源（这里用接口代替 agent）建卡片并移到「In progress」，两个窗口都跟着变
  const card = await createCard(request, board, 'To do', 'Created elsewhere')
  await expect(tileIn(columnOf(a, 'To do'), 'Created elsewhere')).toBeVisible()
  await expect(tileIn(columnOf(b, 'To do'), 'Created elsewhere')).toBeVisible()

  await moveCard(request, card.id, columnId(board, 'In progress'))
  for (const page of [a, b]) {
    await expect(tileIn(columnOf(page, 'In progress'), 'Created elsewhere')).toBeVisible()
    await expect(tileIn(columnOf(page, 'To do'), 'Created elsewhere')).toHaveCount(0)
  }
})
