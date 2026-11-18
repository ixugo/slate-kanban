// 实时渲染编辑器的控制器：把一个 contenteditable 元素变成"输入即渲染"的 Markdown 编辑区。
// 原文是唯一的真相（this.state.value）；DOM 只是它的投影，由 render.ts 逐行画出。
// 除了输入法组合期间，所有改动都在 beforeinput / keydown 里拦下来，交给 lib/md/commands 的纯函数去算，
// 算完重画变过的行并把光标放回去，所以回车续列表、退格退出列表、Ctrl+B 加粗等行为在各浏览器里一致。

import { toggleTaskAt } from '@/lib/md/blocks'
import {
  deleteBackward,
  deleteForward,
  insertNewline,
  insertText,
  pasteText,
  type DeleteUnit,
  type EditState,
} from '@/lib/md/commands'
import { analyze, hiddenLen, lineIndexAt } from '@/lib/md/doc'
import { History, type ChangeKind } from '@/lib/md/history'
import { lineEls, offsetOf, patchLines, readSelection, readValue, writeSelection } from './dom'
import { COMMANDS, keyAction, type Command } from './keymap'
import { LIST_INDENT_EM, TASK_BOX_EM, renderLines } from './render'

/** 创建编辑器时的选项 */
export interface LiveViewOptions {
  /** 全文最多多少个字符；输入、粘贴、命令都不会让文字超过它 */
  max: number
  /** 文字变了（含撤销重做） */
  onChange: (value: string) => void
  /** Ctrl/Cmd+Enter */
  onSubmit?: () => void
  /** Esc；没有提供时 Esc 交还给浏览器 */
  onCancel?: () => void
}

/** 删除类输入事件 → [是否向后删, 删多少] */
const DELETES: Record<string, [boolean, DeleteUnit]> = {
  deleteContentBackward: [false, 'char'],
  deleteContentForward: [true, 'char'],
  deleteWordBackward: [false, 'word'],
  deleteWordForward: [true, 'word'],
  deleteSoftLineBackward: [false, 'line'],
  deleteHardLineBackward: [false, 'line'],
  deleteSoftLineForward: [true, 'line'],
  deleteHardLineForward: [true, 'line'],
  deleteByCut: [false, 'char'],
}

/** 浏览器菜单里的"格式"输入事件 → 对应命令；其余 format* 一律吞掉，不让浏览器往 DOM 里塞 <b> 之类的标签 */
const FORMATS: Record<string, Command> = {
  formatBold: COMMANDS.bold,
  formatItalic: COMMANDS.italic,
  formatUnderline: COMMANDS.underline,
  formatStrikeThrough: COMMANDS.strike,
}

/** 把光标位置挪出每行开头看不见的记号区，并限制在全文范围内 */
function normalize(s: EditState): EditState {
  const doc = analyze(s.value)
  const fix = (p: number): number => {
    const pos = Math.min(Math.max(p, 0), s.value.length)
    const i = lineIndexAt(doc, pos)
    return Math.max(pos, doc.starts[i] + hiddenLen(doc, i))
  }
  const anchor = fix(s.anchor)
  const head = fix(s.head)
  return anchor === s.anchor && head === s.head ? s : { value: s.value, anchor, head }
}

/** 输入框里的实时渲染编辑器 */
export class LiveView {
  private state: EditState
  private history: History
  private shown: string[] = []
  private focused = false
  private composing = false
  private dragging = false
  /** 浏览器自己改了 DOM（我们没拦住的输入类型），input 事件里要从 DOM 重新读回来 */
  private dirty = false
  private readonly cleanups: (() => void)[] = []
  private readonly root: HTMLElement
  private readonly opts: LiveViewOptions

  /** 接管 root，初始文字为 value，光标放在末尾 */
  constructor(root: HTMLElement, value: string, opts: LiveViewOptions) {
    this.root = root
    this.opts = opts
    this.state = { value, anchor: value.length, head: value.length }
    this.history = new History(this.state)
    root.contentEditable = 'true'
    root.setAttribute('role', 'textbox')
    root.setAttribute('aria-multiline', 'true')
    this.draw(true)
    this.bind()
  }

  /** 当前文字和选区 */
  getState(): EditState {
    return this.state
  }

  /** 外部直接换整段文字（比如切换到另一张卡片）；清空撤销历史 */
  setValue(value: string): void {
    if (value === this.state.value) return
    this.state = normalize({ value, anchor: value.length, head: value.length })
    this.history = new History(this.state)
    this.draw(true)
  }

  /** 让编辑器获得焦点，光标回到上次的位置 */
  focus(): void {
    this.focused = true
    this.root.focus()
    this.draw()
    this.paint()
  }

  /** 工具栏按钮用：对当前选区执行一条命令；命令不适用返回 false */
  exec(command: Command): boolean {
    this.pull()
    const next = command(this.state)
    if (next === null) return false
    this.apply(next, 'command')
    return true
  }

  /** 撤销一步 */
  undo(): void {
    this.restore(this.history.undo())
  }

  /** 重做一步 */
  redo(): void {
    this.restore(this.history.redo())
  }

  /** 解除所有事件监听 */
  destroy(): void {
    this.cleanups.forEach((off) => off())
    this.cleanups.length = 0
  }

  /** 挂一个事件监听，并记下怎么摘掉 */
  private on<K extends string>(target: EventTarget, type: K, fn: (e: never) => void): void {
    target.addEventListener(type, fn as unknown as EventListener)
    this.cleanups.push(() => target.removeEventListener(type, fn as unknown as EventListener))
  }

  /** 挂上全部事件 */
  private bind(): void {
    const { root } = this
    const doc = root.ownerDocument
    this.on(root, 'beforeinput', (e: InputEvent) => this.onBeforeInput(e))
    this.on(root, 'input', () => {
      // 浏览器自己改过 DOM（输入法上屏、没拦住的输入类型）：和模型对不上就从 DOM 读回来
      if (this.composing || (!this.dirty && readValue(root) === this.state.value)) return
      this.resync()
    })
    this.on(root, 'keydown', (e: KeyboardEvent) => this.onKeyDown(e))
    this.on(root, 'paste', (e: ClipboardEvent) => {
      e.preventDefault()
      const text = e.clipboardData?.getData('text/plain') ?? ''
      this.pull()
      if (text) this.apply(pasteText(this.state, text, this.opts.max), 'command')
    })
    this.on(root, 'copy', (e: ClipboardEvent) => this.onCopy(e, false))
    this.on(root, 'cut', (e: ClipboardEvent) => this.onCopy(e, true))
    this.on(root, 'drop', (e: Event) => e.preventDefault())
    this.on(root, 'dragstart', (e: Event) => e.preventDefault())
    this.on(root, 'mousedown', (e: MouseEvent) => this.onMouseDown(e))
    this.on(root, 'pointerdown', () => (this.dragging = true))
    const endDrag = (): void => {
      if (!this.dragging) return
      this.dragging = false
      this.onSelectionChange()
    }
    this.on(doc, 'pointerup', endDrag)
    this.on(doc, 'pointercancel', endDrag)
    this.on(root, 'focus', () => {
      this.focused = true
      this.onSelectionChange()
      this.draw()
    })
    this.on(root, 'blur', () => {
      this.focused = false
      this.draw()
    })
    this.on(root, 'compositionstart', () => (this.composing = true))
    this.on(root, 'compositionend', () => {
      this.composing = false
      this.dirty = true
      setTimeout(() => this.flush(), 0)
    })
    this.on(doc, 'selectionchange', () => this.onSelectionChange())
  }

  /** 按当前状态重画变过的行；force 为真时整个重建（DOM 被浏览器改乱了的时候用） */
  private draw(force = false): void {
    const { lo, hi } = { lo: Math.min(this.state.anchor, this.state.head), hi: Math.max(this.state.anchor, this.state.head) }
    const next = renderLines(this.state.value, this.focused ? { lo, hi } : null)
    patchLines(this.root, force ? [] : this.shown, next)
    this.shown = next
    this.root.toggleAttribute('data-empty', this.state.value === '')
  }

  /** 把模型里的选区写到 DOM，并保证光标所在的行看得见 */
  private paint(): void {
    if (!this.focused) return
    writeSelection(this.root, this.state.anchor, this.state.head)
    const doc = analyze(this.state.value)
    lineEls(this.root)[lineIndexAt(doc, this.state.head)]?.scrollIntoView?.({ block: 'nearest' })
  }

  /** 有没读回来的浏览器改动（输入法刚上屏）就先读回来，免得后面的命令基于过时的文字 */
  private flush(): void {
    if (this.dirty && !this.composing) this.resync()
  }

  /** 把 DOM 里的选区读回模型；有变化返回 true */
  private pull(): boolean {
    this.flush()
    const sel = readSelection(this.root)
    if (!sel) return false
    const next = normalize({ value: this.state.value, ...sel })
    if (next.anchor === this.state.anchor && next.head === this.state.head) return false
    this.state = next
    return true
  }

  /** 选区变了：重画记号的显隐。输入法组合和鼠标拖选期间不动 DOM，免得打断 */
  private onSelectionChange(): void {
    if (this.composing || this.dragging) return
    if (!this.pull()) return
    this.draw()
    this.paint()
  }

  /** 采用新状态：记历史、通知外部、重画、放回光标。文字超过上限的改动直接丢弃 */
  private apply(next: EditState, kind: ChangeKind): void {
    const prev = this.state
    if (next.value.length > this.opts.max && next.value.length > prev.value.length) return
    this.state = normalize(next)
    if (next.value !== prev.value) {
      this.history.push(this.state, kind, { anchor: prev.anchor, head: prev.head })
      this.opts.onChange(next.value)
    }
    this.draw()
    this.paint()
  }

  /** 撤销/重做的落地：直接换成历史里的状态 */
  private restore(s: EditState | null): void {
    if (!s) return
    const changed = s.value !== this.state.value
    this.state = normalize(s)
    if (changed) this.opts.onChange(s.value)
    this.draw()
    this.paint()
  }

  /** 从 DOM 把文字和选区整个读回来（输入法结束、浏览器偷偷改了 DOM 时用），再整体重画 */
  private resync(): void {
    if (this.composing) return
    this.dirty = false
    const prev = this.state
    let value = readValue(this.root)
    if (value.length > this.opts.max) value = value.slice(0, this.opts.max).replace(/[\uD800-\uDBFF]$/, '')
    const sel = readSelection(this.root) ?? { anchor: value.length, head: value.length }
    this.state = normalize({ value, anchor: Math.min(sel.anchor, value.length), head: Math.min(sel.head, value.length) })
    if (value !== prev.value) {
      this.history.push(this.state, 'type', { anchor: prev.anchor, head: prev.head })
      this.opts.onChange(value)
    }
    this.draw(true)
    this.paint()
  }

  /** 拦截所有非输入法的输入事件，改成走命令 */
  private onBeforeInput(e: InputEvent): void {
    const type = e.inputType
    if (this.composing || type === 'insertCompositionText' || type === 'deleteCompositionText') return
    this.pull()
    const s = this.state
    const handled = (next: EditState | null, kind: ChangeKind = 'command'): void => {
      e.preventDefault()
      if (next) this.apply(next, kind)
    }
    if (type === 'insertText' && e.data) return handled(insertText(s, e.data, this.opts.max), 'type')
    if (type === 'insertReplacementText') return this.replaceTarget(e)
    if (type === 'insertParagraph') return handled(insertNewline(s, false))
    if (type === 'insertLineBreak') return handled(insertNewline(s, true))
    if (type in DELETES) {
      const [forward, unit] = DELETES[type]
      return handled((forward ? deleteForward : deleteBackward)(s, unit), unit === 'char' ? 'type' : 'command')
    }
    if (type === 'historyUndo' || type === 'historyRedo') {
      e.preventDefault()
      return type === 'historyUndo' ? this.undo() : this.redo()
    }
    if (type.startsWith('insertFrom') || type === 'deleteByDrag') {
      const text = e.dataTransfer?.getData('text/plain')
      return handled(type === 'insertFromPaste' && text ? pasteText(s, text, this.opts.max) : null)
    }
    if (type.startsWith('format')) return handled(FORMATS[type]?.(s) ?? null)
    this.dirty = true
  }

  /** 拼写纠正、自动更正：把浏览器指定的范围换成建议的文字 */
  private replaceTarget(e: InputEvent): void {
    e.preventDefault()
    const text = e.data ?? e.dataTransfer?.getData('text/plain') ?? ''
    const range = e.getTargetRanges?.()[0]
    if (!range) return
    const from = offsetOf(this.root, range.startContainer, range.startOffset)
    const to = offsetOf(this.root, range.endContainer, range.endOffset)
    if (from < 0 || to < 0) return
    this.apply(insertText({ ...this.state, anchor: from, head: to }, text, this.opts.max), 'command')
  }

  /** 键盘快捷键 */
  private onKeyDown(e: KeyboardEvent): void {
    if (e.isComposing || e.keyCode === 229) return
    const action = keyAction(e)
    if (!action) return
    this.pull()
    if ('command' in action) {
      const next = action.command(this.state)
      if (next === null) return
      e.preventDefault()
      this.apply(next, 'command')
      return
    }
    if (action.special === 'cancel' && !this.opts.onCancel) return
    e.preventDefault()
    if (action.special === 'undo') this.undo()
    else if (action.special === 'redo') this.redo()
    else if (action.special === 'submit') this.opts.onSubmit?.()
    else this.opts.onCancel?.()
  }

  /** 复制、剪切：剪贴板里放原文（含 Markdown 记号），这样复制到别处还是 Markdown */
  private onCopy(e: ClipboardEvent, cut: boolean): void {
    this.pull()
    const { value, anchor, head } = this.state
    const text = value.slice(Math.min(anchor, head), Math.max(anchor, head))
    if (!text || !e.clipboardData) return
    e.preventDefault()
    e.clipboardData.setData('text/plain', text)
    if (cut) this.apply(deleteBackward(this.state), 'command')
  }

  /** 鼠标按下：点在任务框上就勾选；按住 Ctrl/Cmd 点链接就在新标签页打开 */
  private onMouseDown(e: MouseEvent): void {
    if (e.button !== 0 || !(e.target instanceof Element)) return
    const link = e.target.closest<HTMLElement>('.md-a[data-href]')
    if (link && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      window.open(link.dataset.href, '_blank', 'noopener,noreferrer')
      return
    }
    const line = e.target.closest<HTMLElement>('.md-line[data-task]')
    if (!line || line.parentElement !== this.root) return
    const em = parseFloat(getComputedStyle(line).fontSize) || 16
    const x = (e.clientX - line.getBoundingClientRect().left) / em
    const left = Number(line.dataset.d) * LIST_INDENT_EM
    if (x < left || x > left + TASK_BOX_EM) return
    e.preventDefault()
    this.pull()
    const index = lineEls(this.root).indexOf(line)
    this.apply({ ...this.state, value: toggleTaskAt(this.state.value, index) }, 'command')
  }
}
