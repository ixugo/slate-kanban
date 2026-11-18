// 卡片弹窗：桌面宽度左右两栏（左边内容、右边「评论和活动」），手机宽度上下排；评论用实时编辑器写，事件和评论排在一起。

import { expect, test, type Page } from '@playwright/test'
import { createBoard, createCard } from './api'
import { columnOf, tileIn } from './board'

const SHOT_DIR = process.env.SLATE_SHOT_DIR

/** 打开看板上的某张卡片，返回弹窗 */
async function openCard(page: Page, boardId: number, title: string) {
  await page.goto(`/b/${boardId}`)
  await tileIn(columnOf(page, 'To do'), title).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('heading', { name: 'Comments and activity' })).toBeVisible()
  return dialog
}

test('桌面宽度：右栏是评论和活动，发评论后与建卡事件排在一起，可收起详情', async ({ page, request }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  const board = await createBoard(request, 'Card pane')
  await createCard(request, board, 'To do', 'Two panes')
  const dialog = await openCard(page, board.id, 'Two panes')

  const desc = await dialog.getByRole('heading', { name: 'Description' }).boundingBox()
  const timeline = await dialog.getByRole('heading', { name: 'Comments and activity' }).boundingBox()
  expect(desc && timeline && timeline.x > desc.x + 300).toBe(true)

  await dialog.getByRole('textbox', { name: 'Comment' }).click()
  await page.keyboard.type('**ship it**')
  await dialog.getByRole('button', { name: 'Send' }).click()
  await expect(dialog.locator('strong', { hasText: 'ship it' })).toBeVisible()
  // 建卡事件也在时间线里
  await expect(dialog.getByText('created the card in')).toBeVisible()
  if (SHOT_DIR) await page.waitForTimeout(600)
  if (SHOT_DIR) await page.screenshot({ path: `${SHOT_DIR}/card-desktop.png` })

  await dialog.getByRole('button', { name: 'Hide details' }).click()
  await expect(dialog.getByText('created the card in')).toHaveCount(0)
  await expect(dialog.locator('strong', { hasText: 'ship it' })).toBeVisible()
})

test('手机宽度：时间线排在内容下面，页面不横向滚动', async ({ page, request }) => {
  await page.setViewportSize({ width: 390, height: 780 })
  const board = await createBoard(request, 'Card phone')
  await createCard(request, board, 'To do', 'Phone card')
  const dialog = await openCard(page, board.id, 'Phone card')

  const desc = await dialog.getByRole('heading', { name: 'Description' }).boundingBox()
  const timeline = await dialog.getByRole('heading', { name: 'Comments and activity' }).boundingBox()
  expect(desc && timeline && timeline.y > desc.y).toBe(true)
  expect(timeline && timeline.x < 60).toBe(true)

  const overflow = await dialog.evaluate((el) => Array.from(el.querySelectorAll('*')).some((n) => n.getBoundingClientRect().right > el.getBoundingClientRect().right + 1))
  expect(overflow).toBe(false)
  if (SHOT_DIR) await page.waitForTimeout(600)
  if (SHOT_DIR) await page.screenshot({ path: `${SHOT_DIR}/card-phone.png` })
})
