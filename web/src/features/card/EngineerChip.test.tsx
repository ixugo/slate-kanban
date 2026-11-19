import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import type { CardDetail, Engineer } from '@/lib/api/types'
import { makeCard } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { EngineerChip } from './EngineerChip'

const engineers: Engineer[] = [
  { id: 1, name: 'Dev A', kind: 'dev', prompt: 'p', avatar: '' },
  { id: 2, name: 'Dev B', kind: 'dev', prompt: 'p', avatar: '' },
  { id: 3, name: 'Rev', kind: 'review', prompt: 'q', avatar: '' },
]
const card = (engineer_id: number) => ({ ...makeCard({ id: 9, version: 4, engineer_id }) }) as unknown as CardDetail

const renderChip = (c: CardDetail, list = engineers) => {
  const onPatch = vi.fn()
  renderApp(<EngineerChip card={c} engineers={list} onPatch={onPatch} />)
  return onPatch
}

describe('EngineerChip', () => {
  it('只列全栈工程师，选中后带版本号提交 engineer_id', async () => {
    const onPatch = renderChip(card(0))
    await userEvent.click(screen.getByRole('button', { name: en.card.field.engineer }))
    expect(screen.getAllByRole('radio').map((r) => r.textContent)).toEqual([en.card.engineerAuto, 'Dev A', 'Dev B'])
    await userEvent.click(screen.getByRole('radio', { name: 'Dev B' }))
    expect(onPatch).toHaveBeenCalledWith({ engineer_id: 2, version: 4 })
  })

  it('已绑定时按钮显示名字，选「随机分配」取消绑定；选当前项不发请求', async () => {
    const onPatch = renderChip(card(1))
    await userEvent.click(screen.getByRole('button', { name: `${en.card.field.engineer}: Dev A` }))
    await userEvent.click(screen.getByRole('radio', { name: 'Dev A' }))
    expect(onPatch).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('button', { name: `${en.card.field.engineer}: Dev A` }))
    await userEvent.click(screen.getByRole('radio', { name: en.card.engineerAuto }))
    expect(onPatch).toHaveBeenCalledWith({ engineer_id: 0, version: 4 })
  })

  it('前端、后端、UI 也算研发类，都能绑定；测试不能', async () => {
    const list: Engineer[] = [
      { id: 1, name: 'Fe', kind: 'frontend', prompt: 'p', avatar: '' },
      { id: 2, name: 'Be', kind: 'backend', prompt: 'p', avatar: '' },
      { id: 3, name: 'Ui', kind: 'ui', prompt: 'p', avatar: '' },
      { id: 4, name: 'Qa', kind: 'review', prompt: 'q', avatar: '' },
    ]
    renderChip(card(0), list)
    await userEvent.click(screen.getByRole('button', { name: en.card.field.engineer }))
    expect(screen.getAllByRole('radio').map((r) => r.textContent)).toEqual([en.card.engineerAuto, 'Fe', 'Be', 'Ui'])
  })

  it('没有全栈工程师且没绑定时不显示', () => {
    renderChip(card(0), [engineers[2]])
    expect(screen.queryByRole('button')).toBeNull()
  })
})
