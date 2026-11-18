// 键盘快捷键：按键 → 编辑命令。Ctrl 和 Cmd 都当作"主修饰键"，所以 Ctrl+B（Windows、Linux）和 Cmd+B（Mac）都是加粗。
// 用 e.code（物理按键）而不是 e.key，这样换了输入法、按住 Shift 后字符变了也认得出来。

import {
  indentList,
  insertCodeBlock,
  insertLink,
  setHeading,
  toggleList,
  toggleQuote,
  toggleWrap,
  type EditState,
} from '@/lib/md/commands'

/** 一条编辑命令：返回 null 表示这个按键当前不适用，交还给浏览器默认处理 */
export type Command = (s: EditState) => EditState | null

/** 快捷键触发的动作：编辑命令，或编辑器级别的动作 */
export type KeyAction = { command: Command } | { special: 'undo' | 'redo' | 'submit' | 'cancel' }

/** 工具栏和快捷键共用的命令表 */
export const COMMANDS: Record<'bold' | 'italic' | 'underline' | 'strike' | 'code' | 'link' | 'bullet' | 'ordered' | 'task' | 'quote' | 'codeBlock', Command> & {
  heading: (level: number) => Command
} = {
  bold: (s) => toggleWrap(s, 'bold'),
  italic: (s) => toggleWrap(s, 'italic'),
  underline: (s) => toggleWrap(s, 'underline'),
  strike: (s) => toggleWrap(s, 'strike'),
  code: (s) => toggleWrap(s, 'code'),
  link: (s) => insertLink(s),
  bullet: (s) => toggleList(s, 'bullet'),
  ordered: (s) => toggleList(s, 'ordered'),
  task: (s) => toggleList(s, 'task'),
  quote: (s) => toggleQuote(s),
  codeBlock: (s) => insertCodeBlock(s),
  heading: (level) => (s) => setHeading(s, level),
}

/** 数字键 7、8、9 对应的列表 */
const LIST_KEYS: Record<string, Command> = { Digit7: COMMANDS.ordered, Digit8: COMMANDS.bullet, Digit9: COMMANDS.task }
/** Mod+字母 对应的行内格式 */
const WRAP_KEYS: Record<string, Command> = { KeyB: COMMANDS.bold, KeyI: COMMANDS.italic, KeyU: COMMANDS.underline, KeyE: COMMANDS.code, KeyK: COMMANDS.link }

/** 判断一个按键事件该做什么；不认识的返回 null */
export function keyAction(e: Pick<KeyboardEvent, 'code' | 'ctrlKey' | 'metaKey' | 'shiftKey' | 'altKey' | 'getModifierState'>): KeyAction | null {
  const mod = e.ctrlKey || e.metaKey
  if (e.getModifierState?.('AltGraph')) return null
  if (!mod && !e.altKey) {
    if (e.code === 'Tab') return { command: (s) => indentList(s, e.shiftKey) }
    if (e.code === 'Escape') return { special: 'cancel' }
    return null
  }
  if (!mod) return null
  if (e.code === 'Enter') return { special: 'submit' }
  if (e.code === 'KeyZ') return { special: e.shiftKey ? 'redo' : 'undo' }
  if (e.code === 'KeyY' && !e.shiftKey && !e.altKey) return { special: 'redo' }
  if (e.altKey && !e.shiftKey) {
    const digit = /^Digit([0-6])$/.exec(e.code)
    if (digit) return { command: COMMANDS.heading(Number(digit[1])) }
    return e.code === 'KeyC' ? { command: COMMANDS.codeBlock } : null
  }
  if (e.altKey) return null
  if (e.shiftKey) {
    if (e.code === 'KeyX') return { command: COMMANDS.strike }
    if (e.code === 'Period') return { command: COMMANDS.quote }
    const list = LIST_KEYS[e.code]
    return list ? { command: list } : null
  }
  const wrap = WRAP_KEYS[e.code]
  return wrap ? { command: wrap } : null
}
