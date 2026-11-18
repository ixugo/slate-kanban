import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi, type FakeCall } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'
import { LabelPicker } from './LabelPicker'
import { LabelsProvider } from './LabelsContext'
import { TagBadge } from './TagBadge'

const LIBRARY = [
  { id: '0000000a', name: 'bug', color: 'red-2' },
  { id: '0000000b', name: '后端', color: '' },
]

/** 渲染选择器；PATCH 看板的请求记在返回的 api 里 */
function setup(tags: string[] = []) {
  const api = installFakeApi({ 'PATCH /api/v1/boards/:id': (call: FakeCall) => ({ body: { id: 1, labels: (call.body as { labels: unknown[] }).labels } }) })
  const onChange = vi.fn()
  renderApp(
    <LabelsProvider boardId={1} labels={LIBRARY} usedTags={['bug', '前端']}>
      <LabelPicker tags={tags} onChange={onChange} />
    </LabelsProvider>,
  )
  return { api, onChange }
}

/** 取出最近一次保存标签库的请求体 */
const savedLabels = (api: ReturnType<typeof setup>['api']) => (api.calls.filter((c) => c.method === 'PATCH')[0].body as { labels: unknown[] }).labels

describe('LabelPicker', () => {
  it('列出标签库和卡片上用过但没登记的标签；点一下加到卡片，再点取消', async () => {
    const { onChange } = setup(['bug'])
    expect(screen.getAllByRole('checkbox').map((el) => el.textContent)).toEqual(['bug', '后端', '前端'])
    expect(screen.getByRole('checkbox', { name: 'bug' })).toHaveAttribute('aria-checked', 'true')
    await userEvent.click(screen.getByRole('checkbox', { name: '后端' }))
    expect(onChange).toHaveBeenLastCalledWith(['bug', '后端'])
    await userEvent.click(screen.getByRole('checkbox', { name: 'bug' }))
    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it('搜索过滤列表；没有匹配时提示；回车把输入的文字当普通标签加到卡片', async () => {
    const { onChange, api } = setup()
    const input = screen.getByLabelText(en.card.field.tags)
    await userEvent.type(input, 'zzz')
    expect(screen.getByText(en.label.empty)).toBeInTheDocument()
    await userEvent.type(input, '{Enter}')
    expect(onChange).toHaveBeenLastCalledWith(['zzz'])
    expect(api.count('PATCH /api/v1/boards/:id')).toBe(0)
  })

  it('新建标签：填名称、选颜色、保存，整体提交标签库并把它加到卡片上', async () => {
    const { api, onChange } = setup()
    await userEvent.click(screen.getByRole('button', { name: en.label.create }))
    await userEvent.type(screen.getByLabelText(en.label.name), '紧急')
    await userEvent.click(screen.getByRole('radio', { name: /Green, medium/ }))
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(['紧急']))
    expect(savedLabels(api)).toEqual([
      { id: '0000000a', name: 'bug', color: 'red-2' },
      { id: '0000000b', name: '后端', color: '' },
      { name: '紧急', color: 'green-2' },
    ])
  })

  it('改名改色按编号提交；删除从标签库里去掉；没登记的标签只能配颜色', async () => {
    const { api } = setup()
    await userEvent.click(screen.getByRole('button', { name: en.label.edit.replace('{{name}}', 'bug') }))
    const name = screen.getByLabelText(en.label.name)
    await userEvent.clear(name)
    await userEvent.type(name, '缺陷')
    await userEvent.click(screen.getByRole('button', { name: en.label.removeColor }))
    await userEvent.click(screen.getByRole('button', { name: en.common.save }))
    await waitFor(() => expect(api.count('PATCH /api/v1/boards/:id')).toBe(1))
    expect(savedLabels(api)).toEqual([
      { id: '0000000a', name: '缺陷', color: '' },
      { id: '0000000b', name: '后端', color: '' },
    ])

    await userEvent.click(await screen.findByRole('button', { name: en.label.edit.replace('{{name}}', '后端') }))
    await userEvent.click(screen.getByRole('button', { name: en.common.delete }))
    await waitFor(() => expect(api.count('PATCH /api/v1/boards/:id')).toBe(2))
    expect((api.calls.filter((c) => c.method === 'PATCH')[1].body as { labels: unknown[] }).labels).toEqual([
      { id: '0000000a', name: 'bug', color: 'red-2' },
    ])

    await userEvent.click(await screen.findByRole('button', { name: en.label.edit.replace('{{name}}', '前端') }))
    expect(screen.getByLabelText(en.label.name)).toHaveAttribute('readonly')
    expect(screen.queryByRole('button', { name: en.common.delete })).toBeNull()
  })
})

describe('TagBadge', () => {
  it('库里配了颜色就用该颜色（不区分大小写），没配或不在库里是默认样式', () => {
    renderApp(
      <LabelsProvider boardId={1} labels={LIBRARY} usedTags={[]}>
        <TagBadge name="BUG" />
        <TagBadge name="后端" />
        <TagBadge name="其他" />
      </LabelsProvider>,
    )
    expect(screen.getByText('BUG')).toHaveStyle({ backgroundColor: '#c9372c', color: '#ffffff' })
    expect(screen.getByText('后端').getAttribute('style')).toBeNull()
    expect(screen.getByText('其他').getAttribute('style')).toBeNull()
  })
})
