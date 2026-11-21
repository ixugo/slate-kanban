import { act, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
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

  it('单击仍查看，双击描述进入编辑，Esc 放弃修改', async () => {
    const onSave = vi.fn()
    render(<DescriptionEditor boardId={1} value="**粗**" onSave={onSave} />)
    await userEvent.click(screen.getByText('粗'))
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
    await userEvent.dblClick(screen.getByText('粗'))
    const editor = await screen.findByRole('textbox', { name: en.card.field.description })
    expect(editor).toHaveTextContent('**粗**')
    fireEvent.keyDown(editor, { key: 'Escape', code: 'Escape' })
    expect(onSave).not.toHaveBeenCalled()
    expect(screen.queryByRole('textbox', { name: en.card.field.description })).toBeNull()
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

  it('卡片引用只链接正文，保留链接、代码和转义字面量', async () => {
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
})
