import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import type { TagInput } from '@/lib/api/types'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { LabelPicker } from './LabelPicker'
import { LabelsProvider } from './LabelsContext'
import { TagBadge } from './TagBadge'

const LIBRARY = [
  { id: 11, text: 'bug', color: 'red-2' },
  { id: 12, text: '后端', color: '' },
  { id: 13, text: '', color: 'green-3' },
]

/** 假服务端：PATCH 看板时给没有主键的新标签分配 100 起的主键，返回完整标签库 */
const saveTags = (call: FakeCall) => {
  const tags = (call.body as { tags: TagInput[] }).tags.map((tag, i) => ({ ...tag, id: tag.id ?? 100 + i }))
  return { body: { id: 1, tags } }
}

/** 渲染选择器；PATCH 看板的请求记在返回的 api 里 */
function setup(tagIds: number[] = []) {
  const api = installFakeApi({ 'PATCH /api/v1/boards/:id': saveTags })
  const onChange = vi.fn()
  renderApp(
    <LabelsProvider boardId={1} tags={LIBRARY}>
      <LabelPicker tagIds={tagIds} onChange={onChange} />
    </LabelsProvider>,
  )
  return { api, onChange }
}

/** 取出第 n 次保存标签库的请求体 */
const savedTags = (api: ReturnType<typeof setup>['api'], n = 0) => (api.calls.filter((c) => c.method === 'PATCH')[n].body as { tags: TagInput[] }).tags

describe('LabelPicker', () => {
  it('列出标签库（含没有文字的标签）；点一下加到卡片，再点取消', async () => {
    const { onChange } = setup([11])
    expect(screen.getAllByRole('checkbox').map((el) => el.textContent)).toEqual(['bug', '后端', ''])
    expect(screen.getByRole('checkbox', { name: 'bug' })).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('checkbox', { name: 'Green, light' })).toHaveAttribute('aria-checked', 'false')
    await userEvent.click(screen.getByRole('checkbox', { name: '后端' }))
    expect(onChange).toHaveBeenLastCalledWith([11, 12])
    await userEvent.click(screen.getByRole('checkbox', { name: 'bug' }))
    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it('搜索过滤列表；没有匹配时提示；回车把输入的文字新建成无颜色标签并加到卡片；已有同文字的标签直接勾选', async () => {
    const { onChange, api } = setup()
    const input = screen.getByLabelText(en.card.field.tags)
    await userEvent.type(input, 'zzz')
    expect(screen.getByText(en.label.empty)).toBeInTheDocument()
    await userEvent.type(input, '{Enter}')
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith([103]))
    expect(savedTags(api).at(-1)).toEqual({ text: 'zzz', color: '' })

    await userEvent.clear(input)
    await userEvent.type(input, 'BUG{Enter}')
    expect(onChange).toHaveBeenLastCalledWith([11])
    expect(api.count('PATCH /api/v1/boards/:id')).toBe(1)
  })

  it('新建标签：文字可以留空，选颜色、保存，整体提交标签库并把它加到卡片上', async () => {
    const { api, onChange } = setup()
    await userEvent.click(screen.getByRole('button', { name: en.label.create }))
    await userEvent.click(screen.getByRole('radio', { name: /Green, medium/ }))
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith([103]))
    expect(savedTags(api)).toEqual([
      { id: 11, text: 'bug', color: 'red-2' },
      { id: 12, text: '后端', color: '' },
      { id: 13, text: '', color: 'green-3' },
      { text: '', color: 'green-2' },
    ])
  })

  it('改文字改颜色按主键提交（文字可以清空）；删除从标签库里去掉', async () => {
    const { api } = setup()
    await userEvent.click(screen.getByRole('button', { name: en.label.edit.replace('{{name}}', 'bug') }))
    const text = screen.getByLabelText(en.label.name)
    await userEvent.clear(text)
    await userEvent.click(screen.getByRole('button', { name: en.label.removeColor }))
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PATCH /api/v1/boards/:id')).toBe(1))
    expect(savedTags(api)).toEqual([
      { id: 11, text: '', color: '' },
      { id: 12, text: '后端', color: '' },
      { id: 13, text: '', color: 'green-3' },
    ])

    await userEvent.click(await screen.findByRole('button', { name: en.label.edit.replace('{{name}}', '后端') }))
    await userEvent.click(screen.getByRole('button', { name: en.common.delete }))
    await waitFor(() => expect(api.count('PATCH /api/v1/boards/:id')).toBe(2))
    expect(savedTags(api, 1)).toEqual([
      { id: 11, text: 'bug', color: 'red-2' },
      { id: 13, text: '', color: 'green-3' },
    ])
  })

  it('卡片已经有 10 个标签时再勾选只提示，不改卡片', async () => {
    const { onChange } = setup([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
    await userEvent.click(screen.getByRole('checkbox', { name: 'bug' }))
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

describe('TagBadge', () => {
  it('有文字的标签用库里配的颜色，没配颜色是默认样式；没有文字的只画颜色条；库里没有的不画', () => {
    renderApp(
      <LabelsProvider boardId={1} tags={LIBRARY}>
        <TagBadge id={11} />
        <TagBadge id={12} />
        <TagBadge id={13} />
        <TagBadge id={99} />
      </LabelsProvider>,
    )
    expect(screen.getByText('bug')).toHaveStyle({ backgroundColor: '#c9372c', color: '#ffffff' })
    expect(screen.getByText('后端').getAttribute('style')).toBeNull()
    expect(screen.getByRole('img', { name: 'Green, light' })).toHaveStyle({ backgroundColor: '#4bce97' })
    expect(screen.getAllByRole('img')).toHaveLength(1)
  })
})
