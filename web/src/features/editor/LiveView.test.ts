import { afterEach, describe, expect, it, vi } from 'vitest'
import { readValue, writeSelection, readSelection } from './dom'
import { LiveView } from './LiveView'

afterEach(() => {
  document.body.innerHTML = ''
})

/** 建一个已获得焦点的编辑器，changes 记录每次 onChange 的文字 */
function setup(value = '', extra: Partial<ConstructorParameters<typeof LiveView>[2]> = {}) {
  const root = document.createElement('div')
  document.body.append(root)
  const changes: string[] = []
  const view = new LiveView(root, value, { max: 1000, onChange: (v) => changes.push(v), ...extra })
  root.dispatchEvent(new Event('focus'))
  return { root, view, changes }
}

/** 模拟用户在 [anchor, head] 之间选中文字，并触发 selectionchange */
function select(root: HTMLElement, anchor: number, head = anchor): void {
  writeSelection(root, anchor, head)
  document.dispatchEvent(new Event('selectionchange'))
}

/** 发一个 beforeinput；返回事件，方便检查有没有被拦下 */
function input(root: HTMLElement, inputType: string, data?: string): InputEvent {
  const e = new InputEvent('beforeinput', { inputType, data, cancelable: true, bubbles: true })
  root.dispatchEvent(e)
  return e
}

/** 发一个键盘事件 */
function key(root: HTMLElement, code: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const e = new KeyboardEvent('keydown', { code, cancelable: true, bubbles: true, ...init })
  root.dispatchEvent(e)
  return e
}

/** 带 clipboardData 的剪贴板事件（jsdom 没有 ClipboardEvent） */
function clip(root: HTMLElement, type: 'paste' | 'copy' | 'cut', text = ''): { e: Event; data: Map<string, string> } {
  const data = new Map<string, string>([['text/plain', text]])
  const e = new Event(type, { cancelable: true, bubbles: true })
  Object.defineProperty(e, 'clipboardData', {
    value: { getData: (t: string) => data.get(t) ?? '', setData: (t: string, v: string) => data.set(t, v) },
  })
  root.dispatchEvent(e)
  return { e, data }
}

describe('渲染与光标', () => {
  it('初始文字按行画出，DOM 文字等于原文', () => {
    const { root } = setup('# 标题\n\n- a\n- b')
    expect(readValue(root)).toBe('# 标题\n\n- a\n- b')
    expect(root.children).toHaveLength(4)
  })

  it('空文档带 data-empty，有字后去掉', () => {
    const { root } = setup('')
    expect(root.hasAttribute('data-empty')).toBe(true)
    input(root, 'insertText', 'a')
    expect(root.hasAttribute('data-empty')).toBe(false)
  })

  it('选区读回模型；落在列表行首看不见的记号里会被挪到正文开头', () => {
    const { root, view } = setup('- a\nb')
    select(root, 0)
    expect(view.getState()).toMatchObject({ anchor: 2, head: 2 })
    expect(readSelection(root)).toEqual({ anchor: 2, head: 2 })
  })

  it('反向选区保持方向', () => {
    const { root, view } = setup('abcd')
    select(root, 3, 1)
    expect(view.getState()).toMatchObject({ anchor: 3, head: 1 })
  })

  it('光标贴着粗体时记号亮出来，离开后藏起来；失去焦点全部藏起来', () => {
    const { root } = setup('a **b** c')
    const hidden = () => root.querySelectorAll('.md-mk.md-hide').length
    select(root, 0)
    expect(hidden()).toBe(2)
    select(root, 4)
    expect(hidden()).toBe(0)
    select(root, 9)
    expect(hidden()).toBe(2)
    select(root, 4)
    root.dispatchEvent(new Event('blur'))
    expect(hidden()).toBe(2)
  })

  it('空行、空列表项里光标落在 <br> 之前', () => {
    const { root } = setup('a\n\n- ')
    select(root, 2)
    const sel = document.getSelection()!
    expect(sel.anchorNode).toBe(root.children[1])
    expect(readSelection(root)).toEqual({ anchor: 2, head: 2 })
    select(root, 5)
    expect(readSelection(root)).toEqual({ anchor: 5, head: 5 })
  })
})

describe('输入与删除', () => {
  it('打字：拦下浏览器默认行为，改模型并通知外部', () => {
    const { root, view, changes } = setup('ac')
    select(root, 1)
    const e = input(root, 'insertText', 'b')
    expect(e.defaultPrevented).toBe(true)
    expect(view.getState()).toEqual({ value: 'abc', anchor: 2, head: 2 })
    expect(changes).toEqual(['abc'])
    expect(readValue(root)).toBe('abc')
    expect(readSelection(root)).toEqual({ anchor: 2, head: 2 })
  })

  it('输入替换选区；超过上限的输入被丢弃', () => {
    const { root, view } = setup('abc', { max: 3 })
    select(root, 0, 3)
    input(root, 'insertText', 'xyz')
    expect(view.getState().value).toBe('xyz')
    select(root, 3)
    input(root, 'insertText', 'q')
    expect(view.getState().value).toBe('xyz')
  })

  it('回车：列表自动续项；空项回车退出列表', () => {
    const { root, view } = setup('- a')
    select(root, 3)
    input(root, 'insertParagraph')
    expect(view.getState().value).toBe('- a\n- ')
    input(root, 'insertParagraph')
    expect(view.getState().value).toBe('- a\n')
    expect(readValue(root)).toBe('- a\n')
  })

  it('Shift+回车（insertLineBreak）在段落里只换一行', () => {
    const { root, view } = setup('ab')
    select(root, 1)
    input(root, 'insertLineBreak')
    expect(view.getState().value).toBe('a\nb')
  })

  it('退格：列表项正文开头退格先去掉记号；有选区时删选区', () => {
    const { root, view } = setup('- a')
    select(root, 2)
    input(root, 'deleteContentBackward')
    expect(view.getState().value).toBe('a')
    select(root, 0, 1)
    input(root, 'deleteContentBackward')
    expect(view.getState().value).toBe('')
  })

  it('Delete、按词删除、删到行首都走命令', () => {
    const { root, view } = setup('abc def')
    select(root, 0)
    input(root, 'deleteContentForward')
    expect(view.getState().value).toBe('bc def')
    select(root, 6)
    input(root, 'deleteWordBackward')
    expect(view.getState().value).toBe('bc ')
    input(root, 'deleteSoftLineBackward')
    expect(view.getState().value).toBe('')
  })

  it('浏览器菜单的格式命令不会往 DOM 里塞标签', () => {
    const { root, view } = setup('abc')
    select(root, 0, 3)
    expect(input(root, 'formatBold').defaultPrevented).toBe(true)
    expect(view.getState().value).toBe('**abc**')
    expect(input(root, 'formatBackColor').defaultPrevented).toBe(true)
    expect(root.querySelector('b, font, span[style]')).toBeNull()
  })

  it('拖放被拒绝', () => {
    const { root, view } = setup('abc')
    expect(input(root, 'insertFromDrop').defaultPrevented).toBe(true)
    expect(input(root, 'deleteByDrag').defaultPrevented).toBe(true)
    expect(view.getState().value).toBe('abc')
  })
})

describe('快捷键', () => {
  it.each([
    ['KeyB', {}, '**abc**'],
    ['KeyI', {}, '*abc*'],
    ['KeyU', {}, '<u>abc</u>'],
    ['KeyX', { shiftKey: true }, '~~abc~~'],
    ['KeyE', {}, '`abc`'],
    ['Digit2', { altKey: true }, '## abc'],
    ['Digit8', { shiftKey: true }, '- abc'],
    ['Digit7', { shiftKey: true }, '1. abc'],
    ['Digit9', { shiftKey: true }, '- [ ] abc'],
    ['Period', { shiftKey: true }, '> abc'],
  ])('Ctrl+%s %j', (code, init, want) => {
    const { root, view } = setup('abc')
    select(root, 0, 3)
    const e = key(root, code, { ctrlKey: true, ...init })
    expect(e.defaultPrevented).toBe(true)
    expect(view.getState().value).toBe(want)
  })

  it('Cmd（metaKey）和 Ctrl 等效', () => {
    const { root, view } = setup('abc')
    select(root, 0, 3)
    key(root, 'KeyB', { metaKey: true })
    expect(view.getState().value).toBe('**abc**')
  })

  it('不认识的组合键交还给浏览器', () => {
    const { root } = setup('abc')
    expect(key(root, 'KeyA', { ctrlKey: true }).defaultPrevented).toBe(false)
    expect(key(root, 'KeyB').defaultPrevented).toBe(false)
  })

  it('Tab 在列表里缩进、在段落里不拦截', () => {
    const { root, view } = setup('- a\n- b\npara')
    select(root, 7)
    expect(key(root, 'Tab').defaultPrevented).toBe(true)
    expect(view.getState().value).toBe('- a\n  - b\npara')
    select(root, view.getState().value.length)
    expect(key(root, 'Tab').defaultPrevented).toBe(false)
  })

  it('Ctrl+Enter 提交，Esc 取消；没有取消回调时 Esc 不拦截', () => {
    const onSubmit = vi.fn()
    const onCancel = vi.fn()
    const a = setup('x', { onSubmit, onCancel })
    key(a.root, 'Enter', { ctrlKey: true })
    key(a.root, 'Escape')
    expect(onSubmit).toHaveBeenCalledOnce()
    expect(onCancel).toHaveBeenCalledOnce()
    const b = setup('x')
    expect(key(b.root, 'Escape').defaultPrevented).toBe(false)
  })

  it('撤销与重做：快捷键和 beforeinput 两条路都通', () => {
    const { root, view, changes } = setup('')
    input(root, 'insertText', 'a')
    input(root, 'insertText', 'b')
    input(root, 'insertParagraph')
    expect(view.getState().value).toBe('ab\n\n')
    key(root, 'KeyZ', { ctrlKey: true })
    expect(view.getState().value).toBe('ab')
    expect(view.getState()).toMatchObject({ anchor: 2, head: 2 })
    key(root, 'KeyZ', { ctrlKey: true })
    expect(view.getState().value).toBe('')
    expect(input(root, 'historyUndo').defaultPrevented).toBe(true)
    key(root, 'KeyY', { ctrlKey: true })
    expect(view.getState().value).toBe('ab')
    input(root, 'historyRedo')
    expect(view.getState().value).toBe('ab\n\n')
    expect(changes.at(-1)).toBe('ab\n\n')
    expect(readValue(root)).toBe('ab\n\n')
  })
})

describe('剪贴板', () => {
  it('粘贴：纯文本，走粘贴命令（选中文字粘网址变链接）', () => {
    const { root, view } = setup('文字')
    select(root, 0, 2)
    const { e } = clip(root, 'paste', 'https://a.com')
    expect(e.defaultPrevented).toBe(true)
    expect(view.getState().value).toBe('[文字](https://a.com)')
  })

  it('复制：剪贴板里是原文，包括看不见的记号', () => {
    const { root } = setup('**a** b')
    select(root, 0, 5)
    const { data, e } = clip(root, 'copy')
    expect(e.defaultPrevented).toBe(true)
    expect(data.get('text/plain')).toBe('**a**')
  })

  it('没有选中文字时复制不拦截；剪切会删除选区', () => {
    const { root, view } = setup('abc')
    select(root, 1)
    expect(clip(root, 'copy').e.defaultPrevented).toBe(false)
    select(root, 0, 2)
    const { data } = clip(root, 'cut')
    expect(data.get('text/plain')).toBe('ab')
    expect(view.getState().value).toBe('c')
  })
})

describe('鼠标', () => {
  it('点任务框切换勾选，点正文不切换', () => {
    const { root, view } = setup('- [ ] a')
    const line = root.children[0] as HTMLElement
    const down = (clientX: number) => {
      const e = new MouseEvent('mousedown', { clientX, button: 0, bubbles: true, cancelable: true })
      line.dispatchEvent(e)
      return e
    }
    expect(down(8).defaultPrevented).toBe(true)
    expect(view.getState().value).toBe('- [x] a')
    expect(down(200).defaultPrevented).toBe(false)
    expect(view.getState().value).toBe('- [x] a')
  })

  it('Ctrl+点链接在新标签页打开', () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    const { root } = setup('[官网](https://a.com)')
    const a = root.querySelector('.md-a') as HTMLElement
    a.dispatchEvent(new MouseEvent('mousedown', { button: 0, ctrlKey: true, bubbles: true, cancelable: true }))
    expect(open).toHaveBeenCalledWith('https://a.com', '_blank', 'noopener,noreferrer')
    open.mockRestore()
  })
})

describe('输入法与浏览器自行改 DOM', () => {
  it('组合期间不碰 DOM、不通知；结束后整体读回', async () => {
    const { root, view, changes } = setup('')
    root.dispatchEvent(new CompositionEvent('compositionstart'))
    expect(input(root, 'insertCompositionText', 'ni').defaultPrevented).toBe(false)
    const line = root.children[0]
    line.replaceChildren(document.createTextNode('你好'))
    document.dispatchEvent(new Event('selectionchange'))
    expect(changes).toEqual([])
    expect(root.children[0]).toBe(line)
    writeSelection(root, 2, 2)
    root.dispatchEvent(new CompositionEvent('compositionend'))
    await new Promise((r) => setTimeout(r, 5))
    expect(view.getState()).toEqual({ value: '你好', anchor: 2, head: 2 })
    expect(changes).toEqual(['你好'])
  })

  it('上屏后马上继续输入，不等定时器也不丢上屏的文字', () => {
    const { root, view } = setup('')
    root.dispatchEvent(new CompositionEvent('compositionstart'))
    root.children[0].replaceChildren(document.createTextNode('你好'))
    writeSelection(root, 2, 2)
    root.dispatchEvent(new CompositionEvent('compositionend'))
    input(root, 'insertText', '!')
    expect(view.getState().value).toBe('你好!')
    expect(readValue(root)).toBe('你好!')
  })

  it('没拦截的输入类型：放行，input 之后从 DOM 读回', () => {
    const { root, view, changes } = setup('ab')
    expect(input(root, 'insertTranspose').defaultPrevented).toBe(false)
    root.children[0].replaceChildren(document.createTextNode('ba'))
    writeSelection(root, 2, 2)
    root.dispatchEvent(new Event('input'))
    expect(view.getState().value).toBe('ba')
    expect(changes).toEqual(['ba'])
  })

  it('浏览器把行拆乱了也能整体恢复', () => {
    const { root, view } = setup('a\nb')
    root.append(document.createElement('div'))
    input(root, 'insertTranspose')
    root.dispatchEvent(new Event('input'))
    expect(view.getState().value).toBe('a\nb\n')
    expect(root.children).toHaveLength(3)
  })
})

describe('生命周期', () => {
  it('setValue 换内容并清空历史；destroy 之后不再响应', () => {
    const { root, view, changes } = setup('a')
    view.setValue('xyz')
    expect(readValue(root)).toBe('xyz')
    key(root, 'KeyZ', { ctrlKey: true })
    expect(view.getState().value).toBe('xyz')
    view.destroy()
    input(root, 'insertText', 'q')
    expect(view.getState().value).toBe('xyz')
    expect(changes).toEqual([])
  })

  it('exec 执行工具栏命令；命令不适用返回 false', () => {
    const { root, view } = setup('abc')
    select(root, 0, 3)
    expect(view.exec((s) => ({ ...s, value: s.value.toUpperCase() }))).toBe(true)
    expect(view.getState().value).toBe('ABC')
    expect(view.exec(() => null)).toBe(false)
  })
})
