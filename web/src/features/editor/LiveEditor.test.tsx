import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { writeSelection } from './dom'
import LiveEditor from './LiveEditor'

/** 渲染编辑器；返回编辑区和 onChange 的记录 */
function setup(value = '') {
  const onChange = vi.fn()
  render(<LiveEditor value={value} onChange={onChange} max={100} placeholder="写点什么" label="描述" />)
  return { editor: screen.getByRole('textbox', { name: '描述' }), onChange }
}

describe('LiveEditor', () => {
  it('有工具栏，编辑区带占位提示和读屏名称', () => {
    const { editor } = setup()
    expect(screen.getByRole('toolbar', { name: en.editor.toolbar })).toBeInTheDocument()
    expect(editor).toHaveAttribute('data-placeholder', '写点什么')
    expect(editor).toHaveAttribute('aria-multiline', 'true')
  })

  it('点工具栏的粗体：把选中文字包起来并通知父组件', async () => {
    const { editor, onChange } = setup('abc')
    editor.dispatchEvent(new Event('focus'))
    writeSelection(editor, 0, 3)
    document.dispatchEvent(new Event('selectionchange'))
    await userEvent.click(screen.getByRole('button', { name: en.editor.bold }))
    expect(onChange).toHaveBeenLastCalledWith('**abc**')
  })

  it('点标题菜单里的二级标题', async () => {
    const { editor, onChange } = setup('abc')
    editor.dispatchEvent(new Event('focus'))
    writeSelection(editor, 1, 1)
    document.dispatchEvent(new Event('selectionchange'))
    await userEvent.click(screen.getByRole('button', { name: en.editor.heading }))
    await userEvent.click(await screen.findByRole('button', { name: en.editor.h2 }))
    expect(onChange).toHaveBeenLastCalledWith('## abc')
  })

  it('帮助里列出快捷键', async () => {
    setup()
    await userEvent.click(screen.getByRole('button', { name: en.editor.help }))
    const tip = await screen.findByText(en.editor.intro)
    const panel = tip.parentElement as HTMLElement
    expect(within(panel).getByText('Ctrl/⌘ B')).toBeInTheDocument()
    expect(within(panel).getByText(en.editor.bold)).toBeInTheDocument()
  })

  it('父组件换了 value，编辑区跟着变', () => {
    const { rerender } = render(<LiveEditor value="a" onChange={() => {}} max={100} placeholder="" label="描述" />)
    rerender(<LiveEditor value="b" onChange={() => {}} max={100} placeholder="" label="描述" />)
    expect(screen.getByRole('textbox', { name: '描述' })).toHaveTextContent('b')
  })
})
