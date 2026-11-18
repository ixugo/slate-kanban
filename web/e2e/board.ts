// 看板页上的定位和操作：按列名找列、按标题找卡片、用真实鼠标拖动卡片。

import type { Locator, Page } from '@playwright/test'

/** 看板上的一列：按列头的名字找 */
export function columnOf(page: Page, name: string): Locator {
  return page.locator('section').filter({ has: page.getByRole('heading', { level: 2, name, exact: true }) })
}

/** 收起成窄边的列（比如默认收起的「完成」）：整条窄边是一个「展开」按钮，名字里带列名和卡片数 */
export function railOf(page: Page, name: string): Locator {
  return page.getByRole('button', { name: new RegExp(`^Expand .${name}.`) })
}

/** 一列里的卡片磁贴（它是一个链接，名字是「编号 标题」），按标题找 */
export function tileIn(column: Locator, title: string): Locator {
  return column.getByRole('link', { name: title })
}

/**
 * 用真实鼠标把一张卡片拖到目标上：按下后先走出传感器要求的 6 像素，再分段移到目标中间，最后松手。
 * offsetY 用来落在目标的上半部分或下半部分（负数偏上），在同一列里调整顺序时要用。
 */
export async function dragTo(page: Page, tile: Locator, target: Locator, offsetY = 0): Promise<void> {
  const from = await tile.boundingBox()
  const to = await target.boundingBox()
  if (!from || !to) throw new Error('卡片或拖动目标不在页面上，没法拖动')
  const startX = from.x + from.width / 2
  const startY = from.y + from.height / 2
  await page.mouse.move(startX, startY)
  await page.mouse.down()
  await page.mouse.move(startX + 12, startY + 12, { steps: 4 })
  await page.mouse.move(to.x + to.width / 2, to.y + to.height / 2 + offsetY, { steps: 24 })
  await page.mouse.up()
}
