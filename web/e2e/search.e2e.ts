// 搜索与筛选：搜索框（关键字和 tag:bug 这样的语法）、「Filter」弹层（标签、优先级），
// 都只留下匹配的卡片，清除后全部回来。

import { expect, test, type APIRequestContext, type Page } from '@playwright/test'
import { createBoard, createCard, uniqueSlug } from './api'

/** 建一个有三张卡片的项目并打开：两张带 bug 标签（一张是高优先级），一张带 docs 标签 */
async function openBoardWithCards(page: Page, request: APIRequestContext) {
  const board = await createBoard(request, `Search ${uniqueSlug()}`)
  await createCard(request, board, 'To do', 'Fix login bug', { tags: ['bug'], priority: 3 })
  await createCard(request, board, 'To do', 'Write docs', { tags: ['docs'] })
  await createCard(request, board, 'In progress', 'Export crashes', { tags: ['bug'] })
  await page.goto(`/b/${board.id}`)
  await expect(page.getByRole('link', { name: 'Write docs' })).toBeVisible()
}

/** 看板上按标题找一张卡片 */
const tile = (page: Page, title: string) => page.getByRole('link', { name: title })

test('搜索框：关键字、搜索语法、没有结果、清空，卡片随之出现或消失', async ({ page, request }) => {
  await openBoardWithCards(page, request)
  const search = page.getByPlaceholder('Search cards ( / )')

  await search.fill('login')
  await expect(tile(page, 'Fix login bug')).toBeVisible()
  await expect(tile(page, 'Write docs')).toBeHidden()
  await expect(tile(page, 'Export crashes')).toBeHidden()
  await expect(page.getByText('1 / 3')).toBeVisible()

  await search.fill('tag:bug')
  await expect(tile(page, 'Fix login bug')).toBeVisible()
  await expect(tile(page, 'Export crashes')).toBeVisible()
  await expect(tile(page, 'Write docs')).toBeHidden()
  await expect(page.getByText('2 / 3')).toBeVisible()

  await search.fill('nothing has this word')
  await expect(page.getByText('No cards match')).toBeVisible()
  await expect(page.getByRole('link', { name: /E\w+-\d+/ })).toHaveCount(0)

  await search.fill('')
  for (const title of ['Fix login bug', 'Write docs', 'Export crashes']) {
    await expect(tile(page, title)).toBeVisible()
  }
})

test('筛选弹层：点选标签和优先级，条件显示在工具栏下面，一键清除后卡片全部回来', async ({ page, request }) => {
  await openBoardWithCards(page, request)

  await page.getByRole('button', { name: 'Filter' }).click()
  await page.getByRole('group', { name: 'Tags' }).getByRole('button', { name: /^bug/ }).click()
  await expect(tile(page, 'Fix login bug')).toBeVisible()
  await expect(tile(page, 'Export crashes')).toBeVisible()
  await expect(tile(page, 'Write docs')).toBeHidden()

  // 再加优先级：只剩高优先级的那一张
  await page.getByRole('group', { name: 'Priority' }).getByRole('button', { name: 'High' }).click()
  await expect(tile(page, 'Fix login bug')).toBeVisible()
  await expect(tile(page, 'Export crashes')).toBeHidden()

  await page.keyboard.press('Escape')
  const active = page.getByRole('group', { name: 'Active filters' })
  await expect(active.getByRole('button', { name: /^Remove filter/ })).toHaveCount(2)

  await active.getByRole('button', { name: 'Clear all' }).click()
  await expect(active).toBeHidden()
  for (const title of ['Fix login bug', 'Write docs', 'Export crashes']) {
    await expect(tile(page, title)).toBeVisible()
  }
})
