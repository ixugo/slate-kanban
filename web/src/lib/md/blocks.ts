// 块级 Markdown：先逐行判断每一行属于哪一种（编辑器按行渲染时要用），
// 再据此组成块树（只读渲染器要用）。两边共用同一套判断，所以输入时和保存后的结构一定一致。
// 只认：#~###### 标题、围栏代码、分隔线、引用、有序/无序/任务列表（可嵌套）和段落；
// 不支持下划线式标题、缩进代码、表格和原始 HTML，它们都按普通段落显示。

export type LineKind = 'blank' | 'fence' | 'code' | 'heading' | 'hr' | 'quote' | 'item' | 'para'

/** 一行的判断结果；编辑器据此把行首的记号和正文分开 */
export interface LineInfo {
  kind: LineKind
  /** 行首记号的长度（含后面的空格；任务项含方框）；正文从这个下标开始。围栏、分隔线整行都是记号 */
  markLen: number
  /** 标题的级别，或引用的层数 */
  level?: number
  /** 列表项：前导空白的宽度（Tab 算 4 列）和字符数 */
  indent?: number
  indentLen?: number
  ordered?: boolean
  /** 有序项的编号 */
  number?: number
  /** 列表项的记号本身，例如 `-`、`12.` */
  bullet?: string
  /** 列表项：记号和它后面空格的结束下标；任务项的方框从这里开始 */
  bulletEnd?: number
  /** 任务项是否已勾选；不是任务项时为 undefined */
  checked?: boolean
  fence?: { open: boolean; char: string; len: number; info: string }
}

export interface ListItem {
  text: string
  /** 这一项在原文里的行号（从 0 数），点任务框时用它定位 */
  line: number
  checked?: boolean
  children: Block[]
}

type ListBlock = { kind: 'list'; ordered: boolean; start: number; items: ListItem[] }

export type Block =
  | { kind: 'code'; info: string; text: string }
  | { kind: 'heading'; level: number; text: string }
  | { kind: 'hr' }
  | { kind: 'quote'; children: Block[] }
  | ListBlock
  | { kind: 'p'; text: string }

/** 引用和列表嵌套的最大层数；更深的按同层或普通文字处理，防止恶意输入撑爆调用栈 */
const MAX_DEPTH = 8
/** 子列表比上一层至少多缩进几列；故意比 CommonMark 宽松，"1. 下面缩进两格"也算子列表。编辑器的缩进、退格也按这个值走 */
export const CHILD_INDENT = 2

const FENCE_OPEN = /^( {0,3})(`{3,}|~{3,})(.*)$/
const HEADING = /^ {0,3}(#{1,6})[ \t]+/
const HR = /^ {0,3}([-*_])(?:[ \t]*\1){2,}[ \t]*$/
const QUOTE = /^ {0,3}(?:>[ \t]?)+/
const ITEM = /^([ \t]*)([-*+]|(\d{1,9})([.)]))[ \t]+/
const TASK = /^\[([ xX])\](?:[ \t]+|$)/
const TASK_LINE = /^((?:[ \t]*>)*[ \t]*(?:[-*+]|\d{1,9}[.)])[ \t]+\[)([ xX])(\](?:[ \t]|$))/

/** 前导空白的宽度：空格算 1 列，Tab 算 4 列 */
function indentWidth(leading: string): number {
  return leading.replace(/\t/g, '    ').length
}

/** 判断 line 是不是围栏代码的结尾：同一种符号、不比开头短、后面只有空白 */
function closesFence(line: string, open: { char: string; len: number }): boolean {
  const body = line.trimStart()
  if (line.length - body.length > 3) return false
  const text = body.trimEnd()
  return text.length >= open.len && [...text].every((c) => c === open.char)
}

/**
 * 判断围栏之外的一行属于哪一种。
 * prev 是上一行的种类，inList 表示上一行属于某个列表项：
 * 紧跟段落文字的列表项要守 CommonMark 的限制（空项、不从 1 开始的有序项不能打断段落），
 * 但列表里的后续项不受限。
 */
function classifyOutside(line: string, prev: LineKind, inList: boolean): LineInfo {
  if (line.trim() === '') return { kind: 'blank', markLen: 0 }
  const fence = FENCE_OPEN.exec(line)
  if (fence && !(fence[2][0] === '`' && fence[3].includes('`'))) {
    return { kind: 'fence', markLen: line.length, fence: { open: true, char: fence[2][0], len: fence[2].length, info: fence[3].trim() } }
  }
  const heading = HEADING.exec(line)
  if (heading) return { kind: 'heading', level: heading[1].length, markLen: heading[0].length }
  if (HR.test(line)) return { kind: 'hr', markLen: line.length }
  const quote = QUOTE.exec(line)
  if (quote) return { kind: 'quote', level: (quote[0].match(/>/g) ?? []).length, markLen: quote[0].length }
  const item = ITEM.exec(line)
  if (item) {
    const ordered = item[3] !== undefined
    const number = ordered ? Number(item[3]) : undefined
    const rest = line.slice(item[0].length)
    const interrupts = rest.trim() !== '' && (!ordered || number === 1)
    if (prev === 'para' && !inList && !interrupts) return { kind: 'para', markLen: 0 }
    const task = TASK.exec(rest)
    return {
      kind: 'item',
      ordered,
      number,
      bullet: item[2],
      indent: indentWidth(item[1]),
      indentLen: item[1].length,
      bulletEnd: item[0].length,
      markLen: item[0].length + (task ? task[0].length : 0),
      checked: task ? task[1] !== ' ' : undefined,
    }
  }
  return { kind: 'para', markLen: 0 }
}

/** 逐行判断每一行属于哪一种；围栏代码里的行一律是 code，不再按别的规则看 */
export function classifyLines(lines: readonly string[]): LineInfo[] {
  const out: LineInfo[] = []
  let open: { char: string; len: number } | null = null
  let prev: LineKind = 'blank'
  let inList = false
  for (const line of lines) {
    let info: LineInfo
    if (open) {
      if (closesFence(line, open)) {
        info = { kind: 'fence', markLen: line.length, fence: { open: false, char: open.char, len: open.len, info: '' } }
        open = null
      } else {
        info = { kind: 'code', markLen: 0 }
      }
    } else {
      info = classifyOutside(line, prev, inList)
      if (info.fence) open = { char: info.fence.char, len: info.fence.len }
    }
    if (info.kind === 'item') inList = true
    else if (info.kind === 'para') inList = inList && /^[ \t]/.test(line)
    else if (info.kind !== 'blank') inList = false
    out.push(info)
    prev = info.kind
  }
  return out
}

/** 去掉引用行最外面一层 `>` 和它后面的一个空格 */
function stripQuote(line: string): string {
  return line.replace(/^ {0,3}>[ \t]?/, '')
}

/** 列表嵌套时记录的一层：缩进、对应的列表块，以及这个列表块挂在哪个数组里 */
interface Level {
  indent: number
  list: ListBlock
  container: Block[]
}

/** 在 container 里新开一个列表块，并作为新的一层返回 */
function openLevel(container: Block[], info: LineInfo): Level {
  const list: ListBlock = { kind: 'list', ordered: info.ordered === true, start: info.number ?? 1, items: [] }
  container.push(list)
  return { indent: info.indent ?? 0, list, container }
}

/**
 * 把一个列表项放进正确的层：缩进比当前层深至少 CHILD_INDENT 列就进上一项的子列表，
 * 比当前层浅至少 2 列就退回外层，其余算同一层的兄弟；同一层换了有序/无序就另起一个列表。
 */
function placeItem(stack: Level[], out: Block[], info: LineInfo, item: ListItem): void {
  const indent = info.indent ?? 0
  if (stack.length === 0) stack.push(openLevel(out, info))
  let k = stack.length - 1
  while (k > 0 && indent < stack[k].indent - 1) k--
  stack.length = k + 1
  const top = stack[k]
  const parent = top.list.items[top.list.items.length - 1]
  if (parent && indent >= top.indent + CHILD_INDENT && stack.length < MAX_DEPTH) {
    stack.push(openLevel(parent.children, info))
  } else if (top.list.ordered !== (info.ordered === true)) {
    stack[k] = openLevel(top.container, info)
  }
  stack[stack.length - 1].list.items.push(item)
}

/** 从 start 起解析一个列表（含嵌套、续行、条目之间的空行），结果放进 out，返回下一行的位置 */
function parseList(lines: readonly string[], infos: readonly LineInfo[], start: number, base: number, out: Block[]): number {
  const stack: Level[] = []
  let last: ListItem | null = null
  let i = start
  while (i < lines.length) {
    const info = infos[i]
    if (info.kind === 'item') {
      last = { text: lines[i].slice(info.markLen).trim(), line: base + i, checked: info.checked, children: [] }
      placeItem(stack, out, info, last)
      i++
    } else if (info.kind === 'para' && last && /^[ \t]/.test(lines[i])) {
      last.text += `\n${lines[i].trim()}`
      i++
    } else if (info.kind === 'blank') {
      let j = i + 1
      while (j < lines.length && infos[j].kind === 'blank') j++
      if (j >= lines.length || infos[j].kind !== 'item') break
      i = j
    } else {
      break
    }
  }
  return i
}

/** 解析 lines 为块列表；base 是 lines[0] 在原文里的行号，depth 是引用的嵌套层数 */
function parseLines(lines: readonly string[], base: number, depth: number): Block[] {
  const infos = classifyLines(lines)
  const blocks: Block[] = []
  let i = 0
  while (i < lines.length) {
    const info = infos[i]
    let j = i + 1
    if (info.kind === 'fence') {
      while (j < lines.length && infos[j].kind === 'code') j++
      blocks.push({ kind: 'code', info: (info.fence?.info ?? '').split(/\s+/)[0], text: lines.slice(i + 1, j).join('\n') })
      if (j < lines.length && infos[j].kind === 'fence') j++
    } else if (info.kind === 'heading') {
      blocks.push({ kind: 'heading', level: info.level ?? 1, text: lines[i].slice(info.markLen).trim() })
    } else if (info.kind === 'hr') {
      blocks.push({ kind: 'hr' })
    } else if (info.kind === 'quote') {
      while (j < lines.length && infos[j].kind === 'quote') j++
      const raw = lines.slice(i, j)
      if (depth >= MAX_DEPTH) blocks.push({ kind: 'p', text: raw.join('\n') })
      else blocks.push({ kind: 'quote', children: parseLines(raw.map(stripQuote), base + i, depth + 1) })
    } else if (info.kind === 'item') {
      j = parseList(lines, infos, i, base, blocks)
    } else if (info.kind === 'para') {
      while (j < lines.length && infos[j].kind === 'para') j++
      blocks.push({ kind: 'p', text: lines.slice(i, j).map((l) => l.trimStart()).join('\n') })
    }
    i = j
  }
  return blocks
}

/** 把整段 Markdown 切成块树；换行统一成 \n */
export function parseBlocks(source: string): Block[] {
  return parseLines(source.replace(/\r\n?/g, '\n').split('\n'), 0, 0)
}

/** 切换原文第 line 行（从 0 数）的任务项勾选状态；不是任务项、越界或在围栏代码里就原样返回 */
export function toggleTaskAt(source: string, line: number): string {
  const lines = source.replace(/\r\n?/g, '\n').split('\n')
  if (!Number.isInteger(line) || line < 0 || line >= lines.length) return source
  if (classifyLines(lines)[line].kind === 'code') return source
  const m = TASK_LINE.exec(lines[line])
  if (!m) return source
  lines[line] = m[1] + (m[2] === ' ' ? 'x' : ' ') + lines[line].slice(m[1].length + 1)
  return lines.join('\n')
}
