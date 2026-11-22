import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import type { CardDetail } from '@/lib/api/types'
import { LabelsProvider } from '@/features/labels/LabelsContext'
import { installFakeApi } from '@/test/fakeApi'
import { makeCard } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { CardMeta } from './CardMeta'

const LIBRARY = [{ id: 11, text: 'bug', color: 'red-2' }]

/** 模拟版本：「标签」按钮在属性一排的左上，设了标签后的「+」按钮在下面的标签区，两处位置明显不同 */
const CHIP_RECT = { x: 20, y: 40, width: 80, height: 32 }
const PLUS_RECT = { x: 20, y: 300, width: 32, height: 32 }

/** 把触发按钮的位置固定下来；jsdom 没有排版，不模拟的话所有位置都是 0 */
function mockButtonRects() {
  return vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
    const isPlus = this instanceof HTMLButtonElement && this.getAttribute('aria-label') === en.card.field.tags
    const isChip = this instanceof HTMLButtonElement && this.textContent === en.card.field.tags
    const r = isPlus ? PLUS_RECT : isChip ? CHIP_RECT : { x: 0, y: 0, width: 0, height: 0 }
    return { ...r, left: r.x, top: r.y, right: r.x + r.width, bottom: r.y + r.height, toJSON: () => r } as DOMRect
  })
}

/** 让 CardMeta 像真实页面一样，勾选标签后把新的 tag_ids 传回来 */
function Harness() {
  const [card, setCard] = useState<CardDetail>({ ...makeCard({ id: 1, tag_ids: [] }), dependencies: [], comments: [], column_instruction: '' } as CardDetail)
  return (
    <LabelsProvider boardId={1} tags={LIBRARY}>
      <CardMeta card={card} depOptions={[]} engineers={[]} renderCardReference={(_, label) => label} onPatch={(p) => setCard({ ...card, ...(p as Partial<CardDetail>) })} />
    </LabelsProvider>
  )
}

/** 取弹层外层定位容器的 transform，也就是它此刻被摆到的位置 */
const popperTransform = () => document.querySelector<HTMLElement>('[data-radix-popper-content-wrapper]')?.style.transform

afterEach(() => vi.restoreAllMocks())

describe('CardMeta 标签选择弹层', () => {
  it('选了标签后入口从按钮挪到标签区，弹层仍留在打开时的位置', async () => {
    installFakeApi({})
    mockButtonRects()
    renderApp(<Harness />)
    await userEvent.click(screen.getByRole('button', { name: en.card.field.tags }))
    const checkbox = await screen.findByRole('checkbox', { name: 'bug' })
    await waitFor(() => expect(popperTransform()).toBeTruthy())
    const before = popperTransform()

    await userEvent.click(checkbox)
    // 现在标签区里出现了「+」按钮，原来的「标签」按钮不见了
    await waitFor(() => expect(screen.getByRole('checkbox', { name: 'bug' })).toBeChecked())
    expect(screen.getByRole('button', { name: en.card.field.tags })).toHaveTextContent('')
    expect(popperTransform()).toBe(before)
  })
})

describe('CardMeta 优先级', () => {
  it('选项前带和看板圆点一致的颜色，选中后按钮上的旗子也变色', async () => {
    installFakeApi({})
    renderApp(<Harness />)
    await userEvent.click(screen.getByRole('button', { name: en.card.field.priority }))
    for (const [name, color] of [['Low', 'bg-p-low'], ['Medium', 'bg-p-mid'], ['High', 'bg-p-high']] as const) {
      const radio = await screen.findByRole('radio', { name })
      expect(radio.querySelector(`.${color}`)).not.toBeNull()
    }
    await userEvent.click(screen.getByRole('radio', { name: 'High' }))
    const trigger = await screen.findByRole('button', { name: new RegExp(`${en.card.field.priority}: High`) })
    expect(trigger.querySelector('svg')).toHaveClass('text-p-high')
  })
})
