// 编辑命令：回车、退格、缩进、加粗、改标题、做列表……所有结构化的编辑动作都写成纯函数。
// 输入输出都是「全文 + 选区」，不碰 DOM，所以所见即所得编辑器和纯文本框共用一份，也能直接单元测试。
// 选区用全文里的字符下标表示：anchor 是起点，head 是光标所在的一端。

import { CHILD_INDENT, type LineInfo } from './blocks'
import { analyze, contentStart, hiddenLen, lineIndexAt, type Doc } from './doc'
import { inlineSource, parseInline, type Inline } from './inline'

/** 编辑状态：全文加选区 */
export interface EditState {
  value: string
  anchor: number
  head: number
}

/** 删除的粒度：一个字、一个词、到行首/行尾 */
export type DeleteUnit = 'char' | 'word' | 'line'

/** 行内格式 */
export type WrapMark = 'bold' | 'italic' | 'strike' | 'underline' | 'code'

/** 列表种类 */
export type ListKind = 'bullet' | 'ordered' | 'task'

/** 一处改动：把 [from, to) 换成 text；一组改动按 from 升序且互不重叠 */
interface Change {
  from: number
  to: number
  text: string
}

const URL_ONLY = /^https?:\/\/[^\s<>()[\]]+$/
const QUOTE_PREFIX = /^ {0,3}>[ \t]?/
const LEADING_SPACE = /^[ \t]*/

const GRAPHEMES = typeof Intl !== 'undefined' && 'Segmenter' in Intl ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null
const WORDS = typeof Intl !== 'undefined' && 'Segmenter' in Intl ? new Intl.Segmenter(undefined, { granularity: 'word' }) : null

/** 选区较小的一端 */
const low = (s: EditState) => Math.min(s.anchor, s.head)
/** 选区较大的一端 */
const high = (s: EditState) => Math.max(s.anchor, s.head)

/** 把改动应用到文本上 */
function applyEdits(value: string, edits: readonly Change[]): string {
  let out = ''
  let at = 0
  for (const e of edits) {
    out += value.slice(at, e.from) + e.text
    at = e.to
  }
  return out + value.slice(at)
}

/**
 * 原文位置 pos 经过这些改动后落在哪：
 * 在被替换范围的起点就留在起点，在终点就落到新文字之后，在中间或恰好是纯插入点时由 assoc 决定靠后（1）还是靠前（-1）。
 */
function mapPos(pos: number, edits: readonly Change[], assoc: 1 | -1): number {
  let delta = 0
  for (const e of edits) {
    if (pos < e.from) break
    if (pos > e.to) {
      delta += e.text.length - (e.to - e.from)
      continue
    }
    if (e.from === e.to) return pos + delta + (assoc > 0 ? e.text.length : 0)
    if (pos === e.from) return pos + delta
    if (pos === e.to) return e.from + delta + e.text.length
    return e.from + delta + (assoc > 0 ? e.text.length : 0)
  }
  return pos + delta
}

/** 应用一组改动，并让选区跟着走：选区端点贴着插入点时落在新文字之后 */
function withEdits(s: EditState, edits: readonly Change[]): EditState {
  const nextLow = mapPos(low(s), edits, 1)
  const nextHigh = mapPos(high(s), edits, 1)
  const forward = s.anchor <= s.head
  return { value: applyEdits(s.value, edits), anchor: forward ? nextLow : nextHigh, head: forward ? nextHigh : nextLow }
}

/** 把 [from, to) 换成 text，光标放到 caret（默认在新文字之后） */
function splice(s: EditState, from: number, to: number, text: string, caret = from + text.length): EditState {
  return { value: s.value.slice(0, from) + text + s.value.slice(to), anchor: caret, head: caret }
}

/** 第 i 行末尾（不含换行）在全文里的下标 */
function lineEnd(doc: Doc, i: number): number {
  return doc.starts[i] + doc.lines[i].length
}

/** 选区覆盖的行号范围；选区非空且右端正好在某行行首时，那一行不算 */
function lineSpan(doc: Doc, a: number, b: number): [number, number] {
  const first = lineIndexAt(doc, a)
  let last = lineIndexAt(doc, b)
  if (b > a && last > first && doc.starts[last] === b) last--
  return [first, last]
}

/** 改块级记号时要处理的行：跳过代码、围栏和分隔线；选了多行时也跳过空行 */
function blockTargets(doc: Doc, a: number, b: number): number[] {
  const [first, last] = lineSpan(doc, a, b)
  const out: number[] = []
  for (let i = first; i <= last; i++) {
    const kind = doc.infos[i].kind
    if (kind === 'code' || kind === 'fence' || kind === 'hr') continue
    if (kind === 'blank' && first !== last) continue
    out.push(i)
  }
  return out
}

/** pos 之前一个"用户看到的字符"的起点；seg 为 null 时退化成按码点 */
export function prevGrapheme(text: string, pos: number, seg: Intl.Segmenter | null = GRAPHEMES): number {
  if (pos <= 0) return 0
  if (seg) {
    let start = 0
    for (const part of seg.segment(text.slice(0, pos))) start = part.index
    return start
  }
  const last = Array.from(text.slice(0, pos)).pop() ?? ''
  return pos - last.length
}

/** pos 之后一个"用户看到的字符"的终点；seg 为 null 时退化成按码点 */
export function nextGrapheme(text: string, pos: number, seg: Intl.Segmenter | null = GRAPHEMES): number {
  if (pos >= text.length) return text.length
  if (seg) {
    const first = seg.segment(text.slice(pos))[Symbol.iterator]().next()
    if (!first.done) return pos + first.value.segment.length
  }
  return pos + (Array.from(text.slice(pos))[0]?.length ?? 0)
}

/** 把一行切成词、空白和标点三类片段，供按词删除用 */
function wordParts(text: string): { index: number; segment: string }[] {
  if (WORDS) return Array.from(WORDS.segment(text), (p) => ({ index: p.index, segment: p.segment }))
  return Array.from(text.matchAll(/\s+|[\p{L}\p{N}_]+|[^\s\p{L}\p{N}_]/gu), (m) => ({ index: m.index, segment: m[0] }))
}

/** 光标之前要删到哪：先跳过紧挨着的空白，再删一个词或一个标点 */
function prevWordStart(text: string, pos: number): number {
  const parts = wordParts(text.slice(0, pos))
  let i = parts.length - 1
  while (i >= 0 && /^\s+$/.test(parts[i].segment)) i--
  return i < 0 ? 0 : parts[i].index
}

/** 光标之后要删到哪：先跳过紧挨着的空白，再删一个词或一个标点 */
function nextWordEnd(text: string, pos: number): number {
  const parts = wordParts(text.slice(pos))
  let i = 0
  while (i < parts.length && /^\s+$/.test(parts[i].segment)) i++
  return i >= parts.length ? text.length : pos + parts[i].index + parts[i].segment.length
}

/** 光标在正文开头时按退格：续行并回上一行；嵌套项先退一级；其余去掉这一行的块记号（引用只去一层） */
function unwrapAtStart(s: EditState, doc: Doc, i: number): EditState {
  const info = doc.infos[i]
  const at = doc.starts[i]
  if (doc.owner[i] >= 0) return splice(s, at - 1, at + hiddenLen(doc, i), '')
  let removed = info.markLen
  if (info.kind === 'item' && (info.indentLen ?? 0) > 0) removed = Math.min(CHILD_INDENT, info.indentLen ?? 0)
  else if (info.kind === 'quote') removed = QUOTE_PREFIX.exec(doc.lines[i])?.[0].length ?? removed
  return splice(s, at, at + removed, '', high(s) - removed)
}

/** 光标在行首时按退格：并到上一行；上一行是分隔线就整条删掉；不把文字并进围栏 */
function joinWithPrevious(s: EditState, doc: Doc, i: number): EditState {
  const at = doc.starts[i]
  if (doc.infos[i - 1].kind === 'hr') return splice(s, doc.starts[i - 1], at, '')
  const touchesFence = doc.infos[i - 1].kind === 'fence' || doc.infos[i].kind === 'fence'
  if (touchesFence && doc.lines[i] !== '') return s
  return splice(s, at - 1, at, '')
}

/** 退格：有选区就删选区；否则按粒度删，行首有特殊的结构处理 */
export function deleteBackward(s: EditState, unit: DeleteUnit = 'char'): EditState {
  const a = low(s)
  const b = high(s)
  if (a !== b) return splice(s, a, b, '')
  if (b === 0) return s
  const doc = analyze(s.value)
  const i = lineIndexAt(doc, b)
  const col = b - doc.starts[i]
  const start = contentStart(doc, i)
  if (start > 0 && col === start) return unwrapAtStart(s, doc, i)
  if (col === 0) return joinWithPrevious(s, doc, i)
  const line = doc.lines[i]
  let from = prevGrapheme(line, col)
  if (unit === 'word') from = prevWordStart(line, col)
  else if (unit === 'line') from = col < start ? 0 : start
  return splice(s, doc.starts[i] + from, b, '')
}

/** 光标在行尾时按 Delete：并入下一行并去掉它的块记号；下一行是分隔线就整条删掉；不把文字并进围栏 */
function joinWithNext(s: EditState, doc: Doc, i: number): EditState {
  const pos = lineEnd(doc, i)
  if (doc.infos[i + 1].kind === 'hr') return splice(s, pos, lineEnd(doc, i + 1), '', pos)
  const touchesFence = doc.infos[i + 1].kind === 'fence' || doc.infos[i].kind === 'fence'
  if (touchesFence && doc.lines[i + 1] !== '') return s
  const strip = doc.lines[i] === '' ? 0 : contentStart(doc, i + 1)
  return splice(s, pos, doc.starts[i + 1] + strip, '', pos)
}

/** Delete：有选区就删选区；否则按粒度向后删 */
export function deleteForward(s: EditState, unit: DeleteUnit = 'char'): EditState {
  const a = low(s)
  const b = high(s)
  if (a !== b) return splice(s, a, b, '')
  if (b >= s.value.length) return s
  const doc = analyze(s.value)
  const i = lineIndexAt(doc, b)
  const col = b - doc.starts[i]
  const line = doc.lines[i]
  if (col === line.length) return joinWithNext(s, doc, i)
  let to = nextGrapheme(line, col)
  if (unit === 'word') to = nextWordEnd(line, col)
  else if (unit === 'line') to = line.length
  return splice(s, b, doc.starts[i] + to, '', b)
}

/** 列表项的下一个记号：缩进、记号、间距都照抄，有序项编号加一，任务项是未完成 */
function nextMarker(info: LineInfo, line: string): string {
  const indentLen = info.indentLen ?? 0
  const bullet = info.bullet ?? '-'
  const gap = LEADING_SPACE.exec(line.slice(indentLen + bullet.length))?.[0] ?? ' '
  const mark = info.ordered ? `${(info.number ?? 0) + 1}${bullet.slice(-1)}` : bullet
  return `${line.slice(0, indentLen)}${mark}${gap}${info.checked === undefined ? '' : '[ ] '}`
}

/** 在空的列表项上回车：嵌套的退一级，顶层的去掉记号变成空行 */
function exitItem(base: EditState, doc: Doc, i: number): EditState {
  const at = doc.starts[i]
  const line = doc.lines[i]
  const indentLen = doc.infos[i].indentLen ?? 0
  const next = indentLen > 0 ? line.slice(Math.min(CHILD_INDENT, indentLen)) : ''
  return splice(base, at, at + line.length, next)
}

/** 列表项（含续行）里回车：Shift+Enter 换成对齐的续行；空项退出；否则在光标处拆成两项 */
function newlineInItem(base: EditState, doc: Doc, i: number, soft: boolean): EditState {
  const pos = base.head
  const owner = doc.owner[i] >= 0 ? doc.owner[i] : i
  const info = doc.infos[owner]
  const line = doc.lines[i]
  const at = doc.starts[i]
  if (doc.owner[i] >= 0) {
    if (soft) return splice(base, pos, pos, `\n${LEADING_SPACE.exec(line)?.[0] ?? ''}`)
    return splice(base, pos, pos, `\n${nextMarker(info, doc.lines[owner])}`)
  }
  const split = Math.max(pos - at, info.markLen) + at
  if (soft) return splice(base, split, split, `\n${' '.repeat(info.bulletEnd ?? 0)}`)
  if (line.slice(info.markLen).trim() === '') return exitItem(base, doc, i)
  return splice(base, split, split, `\n${nextMarker(info, line)}`)
}

/** 引用里回车：续写同样的引用前缀；空的引用行多层就退一层，一层就退出 */
function newlineInQuote(base: EditState, doc: Doc, i: number, soft: boolean): EditState {
  const info = doc.infos[i]
  const line = doc.lines[i]
  const at = doc.starts[i]
  if (!soft && line.slice(info.markLen).trim() === '') {
    const next = (info.level ?? 1) > 1 ? line.slice(QUOTE_PREFIX.exec(line)?.[0].length ?? 0) : ''
    return splice(base, at, at + line.length, next)
  }
  const split = Math.max(base.head - at, info.markLen) + at
  return splice(base, split, split, `\n${line.slice(0, info.markLen)}`)
}

/** 标题里回车：空标题退出；光标在正文开头就在上方加空行；否则光标后面的文字成为普通行 */
function newlineInHeading(base: EditState, doc: Doc, i: number, soft: boolean): EditState {
  const info = doc.infos[i]
  const line = doc.lines[i]
  const at = doc.starts[i]
  const pos = base.head
  if (soft) return splice(base, pos, pos, '\n')
  if (line.slice(info.markLen).trim() === '') return splice(base, at, at + line.length, '')
  if (pos - at <= info.markLen) return splice(base, at, at, '\n', pos + 1)
  return splice(base, pos, pos, '\n')
}

/** 回车：按光标所在行的种类决定怎么换行；Shift+Enter（soft）尽量只换行不开新块 */
export function insertNewline(s: EditState, soft = false): EditState {
  const base = splice(s, low(s), high(s), '')
  const doc = analyze(base.value)
  const pos = base.head
  const i = lineIndexAt(doc, pos)
  const info = doc.infos[i]
  const line = doc.lines[i]
  const col = pos - doc.starts[i]
  if (info.kind === 'item' || doc.owner[i] >= 0) return newlineInItem(base, doc, i, soft)
  if (info.kind === 'quote') return newlineInQuote(base, doc, i, soft)
  if (info.kind === 'heading') return newlineInHeading(base, doc, i, soft)
  if (info.kind === 'code') return splice(base, pos, pos, `\n${LEADING_SPACE.exec(line.slice(0, col))?.[0] ?? ''}`)
  if (info.kind === 'fence' && info.fence?.open && !soft && col === line.length && !doc.infos.slice(i + 1).some((x) => x.kind === 'fence')) {
    return splice(base, pos, pos, `\n\n${info.fence.char.repeat(info.fence.len)}`, pos + 1)
  }
  if (info.kind === 'para' && !soft) return splice(base, pos, pos, '\n\n')
  return splice(base, pos, pos, '\n')
}

/** 回车之外的"改块记号"命令统一从这里取出选区涉及的行 */
function selectedBlockLines(s: EditState, doc: Doc): number[] {
  return blockTargets(doc, low(s), high(s))
}

/** 设置标题级别：同级别再按取消，0 表示改回正文；列表、引用的记号会被标题记号替换 */
export function setHeading(s: EditState, level: number): EditState {
  const doc = analyze(s.value)
  const targets = selectedBlockLines(s, doc)
  const same = targets.length > 0 && targets.every((i) => doc.infos[i].kind === 'heading' && doc.infos[i].level === level)
  const remove = level === 0 || same
  const mark = remove ? '' : `${'#'.repeat(level)} `
  const edits = targets.flatMap((i): Change[] => {
    const { kind, markLen } = doc.infos[i]
    if (remove && kind !== 'heading') return []
    const strip = kind === 'heading' || kind === 'quote' || kind === 'item' ? markLen : 0
    return [{ from: doc.starts[i], to: doc.starts[i] + strip, text: mark }]
  })
  return withEdits(s, edits)
}

/** 这一行是不是已经是指定种类的列表项 */
function isListKind(info: LineInfo, kind: ListKind): boolean {
  if (info.kind !== 'item') return false
  if (kind === 'ordered') return info.ordered === true
  if (kind === 'task') return info.checked !== undefined
  return info.ordered !== true && info.checked === undefined
}

/** 切换列表：选区里的行都已经是这种列表就取消，否则全部改成这种；有序列表重新从 1 编号 */
export function toggleList(s: EditState, kind: ListKind): EditState {
  const doc = analyze(s.value)
  const targets = selectedBlockLines(s, doc)
  const remove = targets.length > 0 && targets.every((i) => isListKind(doc.infos[i], kind))
  let n = 0
  const edits = targets.flatMap((i): Change[] => {
    const info = doc.infos[i]
    const at = doc.starts[i]
    if (remove) return [{ from: at, to: at + info.markLen, text: '' }]
    if (isListKind(info, kind)) {
      n++
      return []
    }
    const keep = info.kind === 'item' || info.kind === 'para' || info.kind === 'blank' ? (info.indentLen ?? LEADING_SPACE.exec(doc.lines[i])?.[0].length ?? 0) : 0
    const end = info.kind === 'heading' || info.kind === 'quote' || info.kind === 'item' ? info.markLen : keep
    const mark = kind === 'ordered' ? `${++n}. ` : kind === 'task' ? '- [ ] ' : '- '
    return [{ from: at + keep, to: at + Math.max(keep, end), text: mark }]
  })
  return withEdits(s, edits)
}

/** 切换引用：选区里的行都已经是引用就各退一层，否则每行前面加一层 */
export function toggleQuote(s: EditState): EditState {
  const doc = analyze(s.value)
  const targets = selectedBlockLines(s, doc)
  const remove = targets.length > 0 && targets.every((i) => doc.infos[i].kind === 'quote')
  const edits = targets.map((i): Change => {
    const at = doc.starts[i]
    if (!remove) return { from: at, to: at, text: '> ' }
    return { from: at, to: at + (QUOTE_PREFIX.exec(doc.lines[i])?.[0].length ?? 0), text: '' }
  })
  return withEdits(s, edits)
}

/** 选区所在的行里有没有代码或围栏：有就不能再套代码块、分隔线 */
function touchesCode(doc: Doc, a: number, b: number): boolean {
  const [first, last] = lineSpan(doc, a, b)
  return doc.infos.slice(first, last + 1).some((x) => x.kind === 'code' || x.kind === 'fence')
}

/** 把选区所在的整行包进围栏代码块；正文里已有三个反引号时用更长的围栏；已经在代码里则不处理 */
export function insertCodeBlock(s: EditState): EditState {
  const a = low(s)
  const b = high(s)
  const doc = analyze(s.value)
  if (touchesCode(doc, a, b)) return s
  const [first, last] = lineSpan(doc, a, b)
  const from = doc.starts[first]
  const to = lineEnd(doc, last)
  const body = s.value.slice(from, to)
  const longest = Math.max(0, ...Array.from(body.matchAll(/`+/g), (m) => m[0].length))
  const fence = '`'.repeat(Math.max(3, longest + 1))
  const shift = fence.length + 1
  const next = splice(s, from, to, `${fence}\n${body}\n${fence}`)
  const clamp = (p: number) => Math.min(p, to) + shift
  const nextAnchor = body === '' ? from + shift : clamp(s.anchor)
  const nextHead = body === '' ? from + shift : clamp(s.head)
  return { ...next, anchor: nextAnchor, head: nextHead }
}

/** 在选区末端所在行之后插入分隔线，光标落到分隔线下面的行；空行直接变成分隔线 */
export function insertDivider(s: EditState): EditState {
  const doc = analyze(s.value)
  if (touchesCode(doc, high(s), high(s))) return s
  const i = lineIndexAt(doc, high(s))
  if (doc.infos[i].kind === 'blank') return splice(s, doc.starts[i], lineEnd(doc, i), '---\n')
  const end = lineEnd(doc, i)
  return splice(s, end, end, '\n---\n')
}

/** 插入链接：选中网址就当地址；选中文字就当链接文字；都没有就插入空链接 */
export function insertLink(s: EditState): EditState {
  const a = low(s)
  const b = high(s)
  const text = s.value.slice(a, b)
  if (a === b) return splice(s, a, a, '[]()', a + 1)
  if (URL_ONLY.test(text)) return splice(s, a, b, `[](${text})`, a + 1)
  return splice(s, a, b, `[${text}]()`, a + text.length + 3)
}

/** Tab 能不能把这一项缩进：上一行必须是列表项（或它的续行），且缩进后最多比上一项深一级 */
function canIndent(doc: Doc, i: number): boolean {
  if (i === 0) return false
  const prev = doc.infos[i - 1].kind === 'item' ? i - 1 : doc.owner[i - 1]
  return prev >= 0 && (doc.infos[i].indent ?? 0) <= (doc.infos[prev].indent ?? 0)
}

/**
 * 列表项缩进（Tab）或退一级（Shift+Tab）。选区里没有列表项时返回 null，让浏览器按默认方式处理；
 * 有列表项但做不了（第一项、越级、已经在顶层）时原样返回，调用方仍应拦住默认行为，免得焦点跳走。
 */
export function indentList(s: EditState, outdent: boolean): EditState | null {
  const doc = analyze(s.value)
  const [first, last] = lineSpan(doc, low(s), high(s))
  const targets: number[] = []
  for (let i = first; i <= last; i++) if (doc.infos[i].kind === 'item') targets.push(i)
  if (targets.length === 0) return null
  if (outdent) {
    const edits = targets
      .filter((i) => (doc.infos[i].indentLen ?? 0) > 0)
      .map((i): Change => ({ from: doc.starts[i], to: doc.starts[i] + Math.min(CHILD_INDENT, doc.infos[i].indentLen ?? 0), text: '' }))
    return edits.length > 0 ? withEdits(s, edits) : s
  }
  if (!canIndent(doc, targets[0])) return s
  return withEdits(
    s,
    targets.map((i): Change => ({ from: doc.starts[i], to: doc.starts[i], text: ' '.repeat(CHILD_INDENT) })),
  )
}

/** 输入文字：替换选区，按 max 截断（不劈开代理对）；已经没有余量就原样返回 */
export function insertText(s: EditState, raw: string, max: number): EditState {
  const text = raw.replace(/\r\n?/g, '\n')
  const a = low(s)
  const b = high(s)
  const room = max - (s.value.length - (b - a))
  if (room <= 0) return s
  let insert = text.slice(0, room)
  if (/[\uD800-\uDBFF]$/.test(insert)) insert = insert.slice(0, -1)
  return splice(s, a, b, insert)
}

/** 粘贴纯文本：统一换行符、按 max 截断（不劈开代理对）；选中文字时粘贴网址会变成链接 */
export function pasteText(s: EditState, raw: string, max: number): EditState {
  const text = raw.replace(/\r\n?/g, '\n')
  const a = low(s)
  const b = high(s)
  const selected = s.value.slice(a, b)
  const url = text.trim()
  const linkable = a !== b && !/[\n[\]]/.test(selected) && URL_ONLY.test(url)
  const link = `[${selected}](${url})`
  if (linkable && link.length <= max - (s.value.length - (b - a))) return splice(s, a, b, link)
  return insertText(s, text, max)
}

/** 行内格式对应的记号，以及怎么认出这种格式的节点 */
const WRAPS: Record<WrapMark, { open: string; close: string; match: (n: Inline) => boolean }> = {
  bold: { open: '**', close: '**', match: (n) => n.kind === 'span' && n.tag === 'strong' },
  italic: { open: '*', close: '*', match: (n) => n.kind === 'span' && n.tag === 'em' },
  strike: { open: '~~', close: '~~', match: (n) => n.kind === 'span' && n.tag === 'del' },
  underline: { open: '<u>', close: '</u>', match: (n) => n.kind === 'span' && n.tag === 'u' },
  code: { open: '`', close: '`', match: (n) => n.kind === 'code' },
}

/** 一段已有格式在全文里的范围：整体 [from, to)，两端记号的长度 */
interface Hit {
  from: number
  to: number
  open: number
  close: number
}

/** 在行内节点里找包住 [a, b] 的、符合 want 的最里面一层，base 是这些节点在全文里的起点 */
function findSpan(nodes: readonly Inline[], base: number, a: number, b: number, want: (n: Inline) => boolean): Hit | null {
  let at = base
  for (const n of nodes) {
    const len = inlineSource([n]).length
    if (n.kind === 'span' || n.kind === 'link' || n.kind === 'code') {
      const inner = at + n.open.length
      if (a >= inner && b <= at + len - n.close.length) {
        const deeper = n.kind === 'code' ? null : findSpan(n.children, inner, a, b, want)
        if (deeper) return deeper
        return want(n) ? { from: at, to: at + len, open: n.open.length, close: n.close.length } : null
      }
    }
    at += len
  }
  return null
}

/** 第 i 行里包住 [a, b] 的某种格式；只看同一行内的 */
function hitInLine(doc: Doc, i: number, a: number, b: number, mark: WrapMark): Hit | null {
  const start = contentStart(doc, i)
  return findSpan(parseInline(doc.lines[i].slice(start)), doc.starts[i] + start, a, b, WRAPS[mark].match)
}

/** 取消格式：左边留着文字就在选区前补一个结束记号，否则删掉开头记号；右边同理 */
function unwrapEdits(hit: Hit, a: number, b: number, mark: WrapMark, whole: boolean): Change[] {
  const { open, close } = WRAPS[mark]
  const left: Change = whole || a <= hit.from + hit.open ? { from: hit.from, to: hit.from + hit.open, text: '' } : { from: a, to: a, text: close }
  const right: Change = whole || b >= hit.to - hit.close ? { from: hit.to - hit.close, to: hit.to, text: '' } : { from: b, to: b, text: open }
  return [left, right]
}

/** 选区在某一行里实际要处理的片段 */
interface Segment {
  line: number
  from: number
  to: number
}

/** 每一行里实际要加格式的片段：跳过代码、空行，记号之后才算正文，两端的空白不包进去 */
function wrapSegments(doc: Doc, a: number, b: number): Segment[] {
  const [first, last] = lineSpan(doc, a, b)
  const out: Segment[] = []
  for (let i = first; i <= last; i++) {
    const kind = doc.infos[i].kind
    if (kind === 'blank' || kind === 'code' || kind === 'fence' || kind === 'hr') continue
    const text = doc.lines[i]
    const lineFrom = doc.starts[i] + contentStart(doc, i)
    let from = Math.max(a, lineFrom)
    let to = Math.min(b, doc.starts[i] + text.length)
    const seg = text.slice(from - doc.starts[i], to - doc.starts[i])
    from += seg.length - seg.trimStart().length
    to -= seg.length - seg.trimEnd().length
    if (to > from) out.push({ line: i, from, to })
  }
  return out
}

/** 选区是不是恰好就是一个完整的某种格式，例如选中了 **ab** 整段 */
function exactSpan(value: string, a: number, b: number, mark: WrapMark): Change[] | null {
  const nodes = parseInline(value.slice(a, b))
  const node = nodes[0]
  if (nodes.length !== 1 || !WRAPS[mark].match(node) || !('open' in node)) return null
  return [
    { from: a, to: a + node.open.length, text: '' },
    { from: b - node.close.length, to: b, text: '' },
  ]
}

/** 一个片段的处理方式：exact 是"恰好一整段格式"要删的两端记号，hit 是"落在某段格式里面"，都没有说明还没加格式 */
interface Plan {
  g: Segment
  exact: Change[] | null
  hit: Hit | null
}

/** 判断片段的现状；选区两端正好停在一段格式的正文边上（多行选区常见）时，把记号也算进来再看是不是恰好一整段 */
function planSegment(s: EditState, doc: Doc, g: Segment, mark: WrapMark): Plan {
  const exact = exactSpan(s.value, g.from, g.to, mark)
  if (exact) return { g, exact, hit: null }
  const hit = hitInLine(doc, g.line, g.from, g.to, mark)
  if (hit) return { g, exact: null, hit }
  const left = hitInLine(doc, g.line, g.from, g.from, mark)
  const right = hitInLine(doc, g.line, g.to, g.to, mark)
  const from = left && g.from === left.from + left.open ? left.from : g.from
  const to = right && g.to === right.to - right.close ? right.to : g.to
  const wide = from !== g.from || to !== g.to ? exactSpan(s.value, from, to, mark) : null
  return { g, exact: wide, hit: null }
}

/** 光标没有选区时的处理：夹在一对空记号中间就删掉；在这种格式里面就整段取消；否则插入一对记号 */
function wrapAtCaret(s: EditState, doc: Doc, mark: WrapMark): EditState {
  const { open, close } = WRAPS[mark]
  const pos = s.head
  const between = pos >= open.length && s.value.slice(pos - open.length, pos) === open && s.value.slice(pos, pos + close.length) === close
  if (between) return splice(s, pos - open.length, pos + close.length, '')
  const hit = hitInLine(doc, lineIndexAt(doc, pos), pos, pos, mark)
  if (hit) return withEdits(s, unwrapEdits(hit, pos, pos, mark, true))
  return splice(s, pos, pos, open + close, pos + open.length)
}

/** 加粗、斜体等：已经有这种格式就取消（选一部分会把它拆开），否则给选区加上；多行选区逐行处理，混合时只给还没加的行加 */
export function toggleWrap(s: EditState, mark: WrapMark): EditState {
  const a = low(s)
  const b = high(s)
  const doc = analyze(s.value)
  if (a === b) return wrapAtCaret(s, doc, mark)
  const { open, close } = WRAPS[mark]
  const segments = wrapSegments(doc, a, b)
  if (segments.length === 0) return s
  const plans = segments.map((g) => planSegment(s, doc, g, mark))
  const unwrap = plans.every((p) => p.exact || p.hit)
  if (!unwrap && mark === 'code' && segments.some((g) => s.value.slice(g.from, g.to).includes('`'))) return s
  const edits = plans.flatMap(({ g, exact, hit }): Change[] => {
    if (unwrap) return exact ?? unwrapEdits(hit as Hit, g.from, g.to, mark, false)
    if (exact || hit) return []
    return [{ from: g.from, to: g.from, text: open }, { from: g.to, to: g.to, text: close }]
  })
  // 处理完后选区正好框住被处理的正文（不含记号和两端的空白），这样再按一次能原路取消
  const start = mapPos(segments[0].from, edits, 1)
  const end = mapPos(segments[segments.length - 1].to, edits, -1)
  const forward = s.anchor <= s.head
  return { value: applyEdits(s.value, edits), anchor: forward ? start : end, head: forward ? end : start }
}
