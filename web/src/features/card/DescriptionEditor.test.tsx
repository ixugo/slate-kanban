import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { readSelection, writeSelection } from '@/features/editor/dom'
import { en } from '@/i18n/locales/en'
import { DescriptionEditor } from './DescriptionEditor'

describe('DescriptionEditor', () => {
  it('空描述可通过键盘触发编辑按钮', async () => {
    render(<DescriptionEditor boardId={1} value="" onSave={() => {}} />)
    expect(screen.getByText(en.card.descPlaceholder)).toBeInTheDocument()
    await userEvent.tab()
    expect(screen.getByRole('button', { name: en.card.descEdit })).toHaveFocus()
    await userEvent.keyboard('{Enter}')
    expect(await screen.findByRole('textbox', { name: en.card.field.description })).toBeInTheDocument()
  })

  it('单击空白描述的占位区可以编辑', async () => {
    render(<DescriptionEditor boardId={1} value="" onSave={vi.fn()} />)
    await userEvent.click(screen.getByText(en.card.descPlaceholder))
    expect(await screen.findByRole('textbox', { name: en.card.field.description })).toBeInTheDocument()
  })

  it('「编辑」按钮悬浮提示可以单击描述编辑', () => {
    render(<DescriptionEditor boardId={1} value="a" onSave={vi.fn()} />)
    expect(screen.getByRole('button', { name: en.card.descEdit })).toHaveAttribute('title', en.card.descEditHint)
  })

  it('单击描述进入编辑，Esc 放弃修改', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value="**粗**" onSave={onSave} />)
    await userEvent.click(screen.getByText('粗'))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(editor).toHaveTextContent('**粗**')
    await act(async () => {
      editor.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: '草稿', cancelable: true, bubbles: true }))
    })
    expect(editor).toHaveTextContent('草稿')
    fireEvent.keyDown(editor, { key: 'Escape', code: 'Escape' })
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const reopened = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(reopened).toHaveTextContent(/^\*\*粗\*\*$/)
  })

  it('没有保存按钮；焦点离开时，没改动不发请求，改了才保存原文', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value="a" onSave={onSave} />)
    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(screen.queryByRole('button', { name: en.common.save })).toBeNull()
    fireEvent.focusOut(editor)
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const again = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(again).not.toBe(editor)
    again.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: 'b', cancelable: true, bubbles: true }))
    await act(async () => {})
    fireEvent.focusOut(again)
    expect(onSave).toHaveBeenCalledWith('ab')
  })

  it('输入法确认候选后立即失焦，保存包含候选字的描述', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value={'甲\n乙'} onSave={onSave} />)
    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    fireEvent.compositionStart(editor)
    editor.children[1].textContent = '乙zhong'
    fireEvent.compositionEnd(editor, { data: '中' })
    fireEvent.focusOut(editor)
    expect(onSave).not.toHaveBeenCalled()
    expect(editor).toBeInTheDocument()

    editor.children[1].textContent = '乙中'
    fireEvent.input(editor, { inputType: 'insertFromComposition', data: '中' })
    await waitFor(() => expect(onSave).toHaveBeenCalledExactlyOnceWith('甲\n乙中'))
  })

  it('回放空描述连续两次拼音上屏，立即续输后保留光标和保存值', async () => {
    const onSave = vi.fn<(value: string) => void>()
    const { rerender } = render(<DescriptionEditor boardId={1} value="" onSave={onSave} />)
    await userEvent.click(screen.getByText(en.card.descPlaceholder))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    const insert = (data: string) => {
      editor.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data, cancelable: true, bubbles: true }))
    }

    act(() => {
      writeSelection(editor, 0, 0)
      fireEvent.compositionStart(editor)
      fireEvent.compositionUpdate(editor, { data: 'zhongwen' })
      editor.children[0].textContent = 'zhongwen'
      writeSelection(editor, 8, 8)
      fireEvent.compositionEnd(editor, { data: '' })
      insert('中文')
      expect(readSelection(editor)).toEqual({ anchor: 2, head: 2 })

      fireEvent.compositionStart(editor)
      fireEvent.compositionUpdate(editor, { data: 'shuru' })
      editor.children[0].textContent = '中文shuru'
      writeSelection(editor, 7, 0)
      fireEvent.compositionEnd(editor, { data: '' })
      insert('输入')
      insert('!')
    })

    expect(editor).toHaveTextContent(/^中文输入!$/)
    expect(readSelection(editor)).toEqual({ anchor: 5, head: 5 })
    fireEvent.focusOut(editor)
    expect(onSave).toHaveBeenCalledExactlyOnceWith('中文输入!')
    expect(editor).not.toBeInTheDocument()

    rerender(<DescriptionEditor boardId={1} value={onSave.mock.calls[0][0]} onSave={onSave} />)
    await userEvent.click(screen.getByRole('button', { name: en.card.descEdit }))
    expect(await screen.findByRole('textbox', { name: en.card.field.description })).toHaveTextContent(/^中文输入!$/)
  })

  it('任务引用只链接正文，保留链接、代码和转义字面量', async () => {
    const onCardReference = vi.fn()
    const value = '见 **#14**、`#15`、[#16](https://a.com/#16)、\\#17。'
    render(
      <DescriptionEditor
        boardId={1}
        value={value}
        onSave={() => {}}
        renderCardReference={(number, label) => <button type="button" onClick={() => onCardReference(number)}>{label}</button>}
      />,
    )

    await userEvent.click(screen.getByRole('button', { name: '#14' }))
    expect(onCardReference).toHaveBeenCalledExactlyOnceWith(14)
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
    expect(screen.getByText('#15').tagName).toBe('CODE')
    expect(screen.getByRole('link', { name: '#16' })).toHaveAttribute('href', 'https://a.com/#16')
    expect(screen.getByText(/#17/)).toBeInTheDocument()
    expect(screen.getAllByRole('button')).toHaveLength(2)
  })

  it('查看状态下点任务框直接保存改过的原文，点链接不进入编辑', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value={'- [ ] 写文档\n\n[官网](https://a.com)'} onSave={onSave} />)
    await userEvent.click(screen.getByRole('checkbox'))
    expect(onSave).toHaveBeenCalledWith('- [x] 写文档\n\n[官网](https://a.com)')
    await userEvent.click(screen.getByRole('link', { name: '官网' }))
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
  })

  it('在卡片描述中渲染管道表格并保留任务框对应的原文', async () => {
    const value = [
      '以下内容在网页上不显示表格，是语法问题还是支持不足?兼容性不够?',
      '-------',
      '',
      '| Slate 工具 | Kanzo 对应工具 | 作用或差异 | Slate 当前视图 |',
      '| --- | --- | --- | --- |',
      '| `whoami` | — | 读取当前身份与服务信息 | 两者都有 |',
      '| `list_boards` | `list_projects` | 列出看板或项目 | 两者都有 |',
      '| `finish_card` | `update_task` + `add_comment` | Slate 一次事务交接，Kanzo 分两步 | 两者都有 |',
      '| `delete_comment` | — | 删除评论 | 仅管理 |',
      '',
      '- [ ] 核对表格',
    ].join('\n')
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value={value} onSave={onSave} />)
    const table = within(screen.getByRole('table'))
    expect(table.getAllByRole('columnheader').map((cell) => cell.textContent)).toEqual([
      'Slate 工具', 'Kanzo 对应工具', '作用或差异', 'Slate 当前视图',
    ])
    expect(table.getAllByRole('row')).toHaveLength(5)
    expect(table.getByRole('cell', { name: 'whoami' }).querySelector('code')).toHaveTextContent('whoami')
    expect(table.getByRole('cell', { name: '读取当前身份与服务信息' })).toBeVisible()
    expect(screen.getByRole('separator')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('checkbox', { name: '核对表格' }))
    expect(onSave).toHaveBeenCalledExactlyOnceWith(value.replace('- [ ] 核对表格', '- [x] 核对表格'))
  })
})
