// 新建项目：从项目列表点「New project」，填表，进到新项目的看板；删除项目：在项目设置里输入项目名称确认。

import { expect, test } from '@playwright/test'
import { createBoard, uniqueSlug } from './api'
import { columnOf, railOf } from './board'

test('新建项目：默认用研发流程模板，进到新项目能看到五列，回到列表也有它', async ({ page }) => {
  const slug = uniqueSlug()
  const name = `Demo ${slug}`
  await page.goto('/')
  await page.getByRole('button', { name: 'New project' }).click()

  const dialog = page.getByRole('dialog', { name: 'New project' })
  await dialog.getByLabel('Name', { exact: true }).fill(name)
  await dialog.getByLabel('Card prefix').fill(slug)
  await expect(dialog.getByRole('switch', { name: 'Use the Development template' })).toBeChecked()
  await dialog.getByRole('button', { name: 'Create project' }).click()

  await expect(page).toHaveURL(/\/b\/\d+$/)
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  for (const column of ['Ideas', 'To do', 'In progress', 'Review']) {
    await expect(columnOf(page, column)).toBeVisible()
  }
  // 「完成」列默认收成窄边，点一下才展开
  await railOf(page, 'Done').click()
  await expect(columnOf(page, 'Done')).toBeVisible()

  await page.goto('/')
  await expect(page.getByRole('link', { name: new RegExp(name) })).toBeVisible()
})

test('新建项目：关掉模板开关得到没有列的空白项目', async ({ page }) => {
  const slug = uniqueSlug()
  await page.goto('/')
  await page.getByRole('button', { name: 'New project' }).click()

  const dialog = page.getByRole('dialog', { name: 'New project' })
  await dialog.getByLabel('Name', { exact: true }).fill(`Blank ${slug}`)
  await dialog.getByLabel('Card prefix').fill(slug)
  await dialog.getByRole('switch', { name: 'Use the Development template' }).click()
  await expect(dialog.getByText('Blank', { exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: 'Create project' }).click()

  await expect(page).toHaveURL(/\/b\/\d+$/)
  await expect(page.getByRole('heading', { name: 'No columns yet' })).toBeVisible()
})

test('删除项目：弹出模态框，输入的名称对上才能删，删完回到列表，接口里也查不到了', async ({ page, request }) => {
  const board = await createBoard(request, `Doomed ${uniqueSlug()}`)
  await page.goto(`/b/${board.id}/settings`)
  await page.getByRole('button', { name: 'Delete project' }).click()

  const dialog = page.getByRole('alertdialog')
  const confirm = dialog.getByLabel('Type the project name to confirm')
  const remove = dialog.getByRole('button', { name: 'Delete', exact: true })
  await expect(remove).toBeDisabled()

  // 大小写不一致、半截名称、卡片编号前缀都不算数
  for (const wrong of [board.name.toLowerCase(), board.name.slice(0, 6), board.slug]) {
    await confirm.fill(wrong)
    await expect(remove).toBeDisabled()
  }

  await confirm.fill(board.name)
  await expect(remove).toBeEnabled()
  await remove.click()

  await expect(page).toHaveURL('/')
  await expect(page.getByRole('link', { name: new RegExp(board.name) })).toHaveCount(0)
  expect((await request.get(`/api/v1/boards/${board.id}`)).status()).toBe(404)
})

test('新建项目：名称为空不能提交，对话框保持打开并提示', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'New project' }).click()

  const dialog = page.getByRole('dialog', { name: 'New project' })
  await dialog.getByRole('button', { name: 'Create project' }).click()

  await expect(dialog.getByRole('alert')).toHaveText('Enter a name.')
  await expect(page).toHaveURL('/')
})
