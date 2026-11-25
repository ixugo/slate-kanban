import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { installFakeApi } from '@/test/fakeApi'
import { readSelection, writeSelection } from './dom'
import LiveEditor, { type LiveEditorDraft } from './LiveEditor'

/** 渲染编辑器；返回编辑区和 onChange 的记录 */
function setup(value = '') {
  const onChange = vi.fn()
  render(<LiveEditor value={value} onChange={onChange} max={100} placeholder="写点什么" label="描述" />)
  return { editor: screen.getByRole('textbox', { name: '描述' }), onChange }
}

describe('LiveEditor', () => {
  it('toolbarOnFocus：没焦点时不显示工具栏，焦点进编辑区才出现，焦点跑到别处又收起', () => {
    render(<LiveEditor value="" onChange={() => {}} max={100} placeholder="写点什么" label="评论" toolbarOnFocus />)
    expect(screen.queryByRole('toolbar')).toBeNull()
    const editor = screen.getByRole('textbox', { name: '评论' })
    fireEvent.focusIn(editor)
    expect(screen.getByRole('toolbar', { name: en.editor.toolbar })).toBeInTheDocument()
    fireEvent.focusOut(editor)
    expect(screen.queryByRole('toolbar')).toBeNull()
  })

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
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ value: '**abc**' }))
  })

  it('点标题菜单里的二级标题', async () => {
    const { editor, onChange } = setup('abc')
    editor.dispatchEvent(new Event('focus'))
    writeSelection(editor, 1, 1)
    document.dispatchEvent(new Event('selectionchange'))
    await userEvent.click(screen.getByRole('button', { name: en.editor.heading }))
    await userEvent.click(await screen.findByRole('button', { name: en.editor.h2 }))
    expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ value: '## abc' }))
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

  it('较早的本地回显保留最新正文和选区，同值的外部回填仍会生效', () => {
    const onChange = vi.fn<(draft: LiveEditorDraft) => void>()
    const editorFor = (draft: LiveEditorDraft) =>
      <LiveEditor {...draft} onChange={onChange} max={100} placeholder="" label="描述" />
    const { rerender } = render(editorFor({ value: '' }))
    const editor = screen.getByRole('textbox', { name: '描述' })
    editor.tabIndex = 0
    editor.focus()
    const type = (data: string) =>
      editor.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data, cancelable: true, bubbles: true }))
    type('中')
    type('文')
    const [first, second] = onChange.mock.calls.map(([draft]) => draft)

    rerender(editorFor(first))
    expect(editor).toHaveTextContent('中文')
    expect(readSelection(editor)).toEqual({ anchor: 2, head: 2 })

    type('!')
    const trailing = onChange.mock.calls[2][0]
    rerender(editorFor(second))
    expect(editor).toHaveTextContent('中文!')
    expect(readSelection(editor)).toEqual({ anchor: 3, head: 3 })

    rerender(editorFor({ value: '中文' }))
    expect(editor).toHaveTextContent(/^中文$/)
    rerender(editorFor(trailing))
    expect(editor).toHaveTextContent(/^中文$/)
  })

  it('切换卡片重新挂载后，前一个编辑器的草稿是外部值', () => {
    const onChange = vi.fn<(draft: LiveEditorDraft) => void>()
    const editorFor = (key: string, draft: LiveEditorDraft) =>
      <LiveEditor key={key} {...draft} onChange={onChange} max={100} placeholder="" label="描述" />
    const { rerender } = render(editorFor('first', { value: '' }))
    const first = screen.getByRole('textbox', { name: '描述' })
    first.dispatchEvent(new InputEvent('beforeinput', { inputType: 'insertText', data: '旧草稿', cancelable: true, bubbles: true }))
    const previousDraft = onChange.mock.calls[0][0]

    rerender(editorFor('second', { value: '另一张卡片' }))
    const second = screen.getByRole('textbox', { name: '描述' })
    expect(second).not.toBe(first)
    expect(second).toHaveTextContent('另一张卡片')
    rerender(editorFor('second', previousDraft))
    expect(second).toHaveTextContent('旧草稿')
  })

  describe('粘贴图片', () => {
    const URL = '/api/v1/attachments/0123456789abcdef0123456789abcdef'

    /** 渲染带看板编号的编辑器（光标在末尾），返回编辑区和 onChange 记录 */
    function setupBoard(value = '前') {
      const onChange = vi.fn()
      render(<LiveEditor value={value} onChange={onChange} max={500} placeholder="" label="描述" boardId={7} />)
      const editor = screen.getByRole('textbox', { name: '描述' })
      editor.dispatchEvent(new Event('focus'))
      return { editor, onChange }
    }

    /** 往编辑区派发一次粘贴事件：text 是纯文字，files 是剪贴板里的文件 */
    function paste(editor: HTMLElement, text: string, files: File[]) {
      const e = new Event('paste', { bubbles: true, cancelable: true })
      Object.defineProperty(e, 'clipboardData', { value: { getData: () => text, files } })
      editor.dispatchEvent(e)
      return e
    }

    const png = () => new File([new Uint8Array([137, 80, 78, 71])], 'image.png', { type: 'image/png' })

    it('只有图片：上传到看板，传完在光标处插入图片语法', async () => {
      const api = installFakeApi({ 'POST /api/v1/boards/:id/attachments': { token: 't', url: URL, content_type: 'image/png', size: 4 } })
      const { editor, onChange } = setupBoard()
      const e = paste(editor, '', [png()])
      expect(e.defaultPrevented).toBe(true)
      await waitFor(() => expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ value: `前![image](${URL})` })))
      expect(api.calls).toHaveLength(1)
      expect(api.calls[0].path).toBe('/api/v1/boards/7/attachments')
      expect((api.calls[0].body as { file: File }).file).toBeInstanceOf(File)
      await waitFor(() => expect(screen.queryByRole('status')).toBeNull())
    })

    it('图片和文字同时在剪贴板里：按文字粘贴，不上传', () => {
      const api = installFakeApi({})
      const { editor, onChange } = setupBoard()
      paste(editor, '表格', [png()])
      expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ value: '前表格' }))
      expect(api.calls).toHaveLength(0)
    })

    it('没有看板编号时不处理图片', () => {
      const api = installFakeApi({})
      const onChange = vi.fn()
      render(<LiveEditor value="前" onChange={onChange} max={500} placeholder="" label="描述" />)
      paste(screen.getByRole('textbox', { name: '描述' }), '', [png()])
      expect(api.calls).toHaveLength(0)
      expect(onChange).not.toHaveBeenCalled()
    })

    it('上传失败：不插入任何内容，提示消失', async () => {
      installFakeApi({ 'POST /api/v1/boards/:id/attachments': () => ({ status: 413, body: { reason: 'ErrAttachmentTooLarge', msg: '图片太大' } }) })
      const { editor, onChange } = setupBoard()
      paste(editor, '', [png()])
      await waitFor(() => expect(screen.queryByRole('status')).toBeNull())
      expect(onChange).not.toHaveBeenCalled()
    })
  })
})
