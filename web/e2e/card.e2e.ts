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

/** 一张 1×1 的 PNG，用来模拟剪贴板里的图片 */
const TINY_PNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=='

test('在评论框里粘贴图片：自动上传，发出去后评论里显示这张图片', async ({ page, request }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  const board = await createBoard(request, 'Paste image')
  await createCard(request, board, 'To do', 'With picture')
  const dialog = await openCard(page, board.id, 'With picture')

  const editor = dialog.getByRole('textbox', { name: 'Comment' })
  await editor.click()
  // 模拟"只有图片文件"的粘贴：没有文字，剪贴板里是一个 PNG 文件
  await editor.evaluate((el, b64) => {
    const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0))
    const data = new DataTransfer()
    data.items.add(new File([bytes], 'image.png', { type: 'image/png' }))
    el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }))
  }, TINY_PNG)

  // 上传完成后编辑区里有图片预览，原文是指向本服务附件的图片语法
  const preview = editor.locator('img.md-img')
  await expect(preview).toHaveAttribute('src', /^\/api\/v1\/attachments\/[0-9a-f]{32}$/)
  await expect.poll(() => preview.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(1)

  await dialog.getByRole('button', { name: 'Send' }).click()
  const sent = dialog.locator('ul img[alt="image"]')
  await expect(sent).toBeVisible()
  await expect.poll(() => sent.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBe(1)
})

test('评论框的占位提示按正常排版（带内边距），不是贴着编辑区左上角的绝对定位', async ({ page, request }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  const board = await createBoard(request, 'Placeholder align')
  await createCard(request, board, 'To do', 'Hint card')
  const dialog = await openCard(page, board.id, 'Hint card')

  const editor = dialog.getByRole('textbox', { name: 'Comment' })
  await expect(editor).toHaveAttribute('data-empty', '')
  // 占位提示是 ::before 伪元素，量不到它的位置；绝对定位会无视内边距，所以确认它不是绝对定位且参与排版
  const hint = await editor.evaluate((el) => {
    const css = getComputedStyle(el, '::before')
    return { content: css.content, position: css.position, display: css.display }
  })
  expect(hint.content).toContain('Write a comment')
  expect(hint.position).toBe('static')
  expect(hint.display).toBe('block')
})
