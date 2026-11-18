// 导出：项目设置里的「Export」页签能真的下载 JSON 和 Markdown；看板顶栏不再有导出入口。

import { readFile } from 'node:fs/promises'
import { expect, test, type Page } from '@playwright/test'
import { createBoard, createCard, uniqueSlug } from './api'

/** 点「Export」页签里某一行的下载链接，等浏览器真的开始下载，返回文件名和内容 */
async function downloadFrom(page: Page, rowText: string): Promise<{ name: string; text: string }> {
  const row = page.getByRole('listitem').filter({ hasText: rowText })
  const [download] = await Promise.all([page.waitForEvent('download'), row.getByRole('link', { name: 'Download' }).click()])
  const path = await download.path()
  return { name: download.suggestedFilename(), text: await readFile(path, 'utf8') }
}

test('导出：设置页的 Export 页签下载 JSON 和 Markdown，内容里有项目和卡片，顶栏没有导出按钮', async ({ page, request }) => {
  const board = await createBoard(request, `Export ${uniqueSlug()}`)
  await createCard(request, board, 'To do', 'Ship the release', { tags: ['launch'] })

  await page.goto(`/b/${board.id}`)
  await expect(page.getByRole('link', { name: 'Ship the release' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Export' })).toHaveCount(0)

  await page.getByRole('link', { name: 'Project settings' }).click()
  await expect(page).toHaveURL(`/b/${board.id}/settings`)
  await page.getByRole('tab', { name: 'Export' }).click()

  const json = await downloadFrom(page, 'Complete data')
  expect(json.name).toMatch(/^[a-z0-9-]+-\d{8}\.json$/)
  const parsed = JSON.parse(json.text)
  expect(parsed.format).toBe('slate-board-export')
  expect(parsed.board.name).toBe(board.name)
  expect(parsed.cards.map((c: { title: string }) => c.title)).toEqual(['Ship the release'])

  const markdown = await downloadFrom(page, 'A readable document')
  expect(markdown.name).toMatch(/^[a-z0-9-]+-\d{8}\.md$/)
  expect(markdown.text).toContain(board.name)
  expect(markdown.text).toContain('Ship the release')
})
