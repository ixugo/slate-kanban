// 五种界面语言：在顶栏的语言菜单里逐个切换，页面文字随之变化，刷新后仍是所选语言。
// 期望的文字直接取自 src/i18n/locales，翻译改了这里不用跟着改。

import { expect, test } from '@playwright/test'
import { en } from '../src/i18n/locales/en'
import { ja } from '../src/i18n/locales/ja'
import { ko } from '../src/i18n/locales/ko'
import { zhCN } from '../src/i18n/locales/zh-CN'
import { zhTW } from '../src/i18n/locales/zh-TW'
import { createBoard, uniqueSlug } from './api'

// 切换的顺序保证每一步都真的换了语言（最开始是英文），最后停在韩文：它不是默认语言，刷新后还在才说明记住了选择
const LANGUAGES = [
  { code: 'zh-CN', messages: zhCN },
  { code: 'zh-TW', messages: zhTW },
  { code: 'en', messages: en },
  { code: 'ja', messages: ja },
  { code: 'ko', messages: ko },
] as const

test('五种语言逐个切换：标题、按钮和 html 的 lang 随之变化，刷新后保持最后选的语言', async ({ page, request }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(en.boards.title)
  // 语言菜单的名字会随语言变，所以按「里面有这五个选项」来找它（页面上还有切换项目的下拉）
  const languageMenu = page.locator('select:has(option[value="zh-CN"])')

  for (const { code, messages } of LANGUAGES) {
    await languageMenu.selectOption(code)
    await expect(page.getByRole('heading', { level: 1 })).toHaveText(messages.boards.title)
    await expect(page.getByRole('button', { name: messages.boards.new })).toBeVisible()
    await expect(page.locator('html')).toHaveAttribute('lang', code)
  }

  await page.reload()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(ko.boards.title)
  await expect(page.locator('html')).toHaveAttribute('lang', 'ko')

  // 进到看板页，那里的文字也是韩文（语言包是按需加载的，这里同时验证了加载）
  const board = await createBoard(request, `Lang ${uniqueSlug()}`)
  await page.goto(`/b/${board.id}`)
  await expect(page.getByPlaceholder(ko.search.placeholder)).toBeVisible()
  await expect(page.getByRole('button', { name: ko.filter.button })).toBeVisible()
})
