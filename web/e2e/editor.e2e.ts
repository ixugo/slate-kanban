// 卡片描述的实时渲染编辑器：在真实 Chrome 里验证输入即渲染、快捷键、列表续写、输入法、任务框和手机宽度。

import { expect, test, type Page } from '@playwright/test'
import { createBoard, createCard, getCard } from './api'
import { columnOf, tileIn } from './board'

/** 打开卡片并进入描述编辑，返回编辑区 */
async function openEditor(page: Page, boardId: number, title: string) {
  await page.goto(`/b/${boardId}`)
  await tileIn(columnOf(page, 'To do'), title).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('button', { name: 'Edit', exact: true }).click()
  const editor = dialog.getByRole('textbox', { name: 'Description' })
  await expect(editor).toBeFocused()
  return { dialog, editor }
}

/** 编辑区里第 n 行（从 0 数） */
const lineAt = (editor: ReturnType<Page['locator']>, n: number) => editor.locator('> .md-line').nth(n)

test('输入即渲染：标题、粗体、列表自动续写，Ctrl+Enter 保存的是原文', async ({ page, request }) => {
  const board = await createBoard(request, 'Editor')
  const card = await createCard(request, board, 'To do', 'Write docs')
  const { dialog, editor } = await openEditor(page, board.id, 'Write docs')

  await page.keyboard.type('# Hello')
  await expect(lineAt(editor, 0)).toHaveClass(/md-h1/)
  await page.keyboard.press('Enter')
  await page.keyboard.type('**bold** and *it*')
  await expect(editor.locator('.md-b')).toHaveText('bold')
  await expect(editor.locator('.md-i')).toHaveText('it')
  // 光标离开粗体后记号藏起来：看得见的文字里没有星号
  await expect(editor.locator('.md-mk.md-hide')).not.toHaveCount(0)
  await page.keyboard.press('Enter')
  await page.keyboard.type('- one')
  await page.keyboard.press('Enter')
  await page.keyboard.type('two')
  await page.keyboard.press('Enter')
  await page.keyboard.press('Enter')
  await page.keyboard.type('end')

  const value = await editor.evaluate((el) => Array.from(el.children).map((c) => c.textContent).join('\n'))
  expect(value).toBe('# Hello\n**bold** and *it*\n\n- one\n- two\nend')

  await page.keyboard.press('Control+Enter')
  await expect(editor).toHaveCount(0)
  await expect.poll(async () => (await getCard(request, card.id)).description).toBe(value)
  await expect(dialog.getByRole('heading', { name: 'Hello' })).toBeVisible()
})

test('快捷键：Ctrl+B / I / U 包住选中文字，再按一次取消，Ctrl+Z 撤销', async ({ page, request }) => {
  const board = await createBoard(request, 'Keys')
  await createCard(request, board, 'To do', 'Shortcuts')
  const { editor } = await openEditor(page, board.id, 'Shortcuts')
  const text = () => editor.evaluate((el) => Array.from(el.children).map((c) => c.textContent).join('\n'))

  await page.keyboard.type('abc')
  await page.keyboard.press('Shift+Home')
  await page.keyboard.press('Control+b')
  expect(await text()).toBe('**abc**')
  await expect(editor.locator('.md-b')).toHaveText('abc')
  await page.keyboard.press('Control+b')
  expect(await text()).toBe('abc')
  await page.keyboard.press('Control+u')
  expect(await text()).toBe('<u>abc</u>')
  await page.keyboard.press('Control+i')
  expect(await text()).toBe('<u>*abc*</u>')
  await page.keyboard.press('Control+z')
  expect(await text()).toBe('<u>abc</u>')
  await page.keyboard.press('Control+Shift+z')
  expect(await text()).toBe('<u>*abc*</u>')
})

test('退格：列表项开头退一格先去掉圆点；Tab 缩进成子列表', async ({ page, request }) => {
  const board = await createBoard(request, 'List')
  await createCard(request, board, 'To do', 'Lists')
  const { editor } = await openEditor(page, board.id, 'Lists')
  const text = () => editor.evaluate((el) => Array.from(el.children).map((c) => c.textContent).join('\n'))

  await page.keyboard.type('- a')
  await page.keyboard.press('Enter')
  await page.keyboard.type('b')
  await page.keyboard.press('Tab')
  expect(await text()).toBe('- a\n  - b')
  await expect(lineAt(editor, 1)).toHaveAttribute('data-d', '1')
  await page.keyboard.press('Shift+Tab')
  expect(await text()).toBe('- a\n- b')
  await page.keyboard.press('Home')
  await page.keyboard.press('Backspace')
  expect(await text()).toBe('- a\nb')
})

test('输入法组合：组合期间不打断，上屏后文字正确', async ({ page, request }) => {
  const board = await createBoard(request, 'IME')
  await createCard(request, board, 'To do', 'Ime')
  const { editor } = await openEditor(page, board.id, 'Ime')
  const cdp = await page.context().newCDPSession(page)

  await page.keyboard.type('# ')
  await cdp.send('Input.imeSetComposition', { text: 'ni', selectionStart: 2, selectionEnd: 2 })
  await cdp.send('Input.imeSetComposition', { text: '你', selectionStart: 1, selectionEnd: 1 })
  await cdp.send('Input.insertText', { text: '你好' })
  await page.keyboard.type(' **粗**')
  const value = await editor.evaluate((el) => Array.from(el.children).map((c) => c.textContent).join('\n'))
  expect(value).toBe('# 你好 **粗**')
  await expect(editor.locator('.md-b')).toHaveText('粗')
})

test('任务框：点一下勾选，再点取消', async ({ page, request }) => {
  const board = await createBoard(request, 'Task')
  await createCard(request, board, 'To do', 'Tasks')
  const { editor } = await openEditor(page, board.id, 'Tasks')

  await page.keyboard.type('- [ ] todo')
  const line = lineAt(editor, 0)
  const box = (await line.boundingBox())!
  const em = await line.evaluate((el) => parseFloat(getComputedStyle(el).fontSize))
  await page.mouse.click(box.x + em * 0.7, box.y + em * 0.8)
  await expect(line).toHaveAttribute('data-task', '1')
  await page.mouse.click(box.x + em * 0.7, box.y + em * 0.8)
  await expect(line).toHaveAttribute('data-task', '0')
})

test.describe('手机宽度', () => {
  test.use({ viewport: { width: 390, height: 780 }, hasTouch: true, isMobile: true })

  test('工具栏横向可滚动，页面不出现横向滚动条，编辑区可用', async ({ page, request }) => {
    const board = await createBoard(request, 'Phone')
    await createCard(request, board, 'To do', 'Mobile card')
    await page.goto(`/b/${board.id}`)
    await tileIn(columnOf(page, 'To do'), 'Mobile card').tap()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('button', { name: 'Edit', exact: true }).tap()
    const editor = dialog.getByRole('textbox', { name: 'Description' })
    await expect(editor).toBeVisible()
    await editor.tap()
    await page.keyboard.type('## Mobile **ok**')
    await expect(lineAt(editor, 0)).toHaveClass(/md-h2/)

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
    expect(overflow).toBeLessThanOrEqual(0)
    const toolbar = dialog.getByRole('toolbar').first()
    const button = await toolbar.getByRole('button', { name: 'Bold' }).boundingBox()
    expect(button!.width).toBeGreaterThanOrEqual(40)
  })
})
