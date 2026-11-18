// 把 Markdown 全文画成"一行一个 div"的 HTML 字符串，供实时渲染编辑器使用。
// 铁律：每个 div 的 textContent 恰好等于原文那一行，Markdown 记号都是真实文字，
// 只是用 CSS 藏起来（md-hide），光标碰到才亮出来。这样 DOM 和原文永远一一对应，不会丢字。
// 纯函数：同样的 (全文, 选区) 一定得到同样的 HTML，方便测试，也方便逐行对比后只替换变过的行。

import { parseBlocks, type Block } from '@/lib/md/blocks'
import { analyze, hiddenLen, lineIndexAt, type Doc } from '@/lib/md/doc'
import { inlineSource, parseInline, type Inline } from '@/lib/md/inline'

/** 选区的绝对位置（含两端）；传 null 表示编辑器没有焦点，所有记号都藏起来 */
export interface Span {
  lo: number
  hi: number
}

/** 列表每缩一层往右多少 em；样式表里的 --md-indent 必须是同一个数，点任务框时也靠它换算坐标 */
export const LIST_INDENT_EM = 1.5
/** 任务框的宽度（em），只用来判断鼠标是否点在框上 */
export const TASK_BOX_EM = 1.3

/** 无序列表各层的圆点，循环使用 */
const BULLETS = ['•', '◦', '▪']
/** 引用最多画几道竖线，更深的和第 3 层一样 */
const MAX_QUOTE_BARS = 3

/** 一行里的一小段：text 是原文，cls 是样式类，hidden 表示当前被藏起来（不占版面） */
interface Part {
  text: string
  cls: string
  hidden: boolean
  href?: string
  /** 图片预览：这一段本身没有文字，只在原文记号后面画一张图 */
  img?: { src: string; alt: string }
}

/** 要按行内语法一起解析的一组行：lines 是行号，cols 是每行正文从第几个字符开始 */
interface Unit {
  lines: number[]
  cols: number[]
}

const SPAN_CLASS = { strong: 'md-b', em: 'md-i', del: 'md-s', u: 'md-u' } as const

/** 转义文字，防止用户内容被当成标签 */
function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

/** 记号片段：选区碰到所属的格式时亮出来，否则藏起来 */
function mark(text: string, on: boolean): Part {
  return { text, cls: on ? 'md-mk' : 'md-mk md-hide', hidden: !on }
}

/** 把一组行的正文按行内语法解析，返回每一行的片段；格式可以跨行（段落里的软换行） */
function renderUnit(doc: Doc, unit: Unit, sel: Span | null): Part[][] {
  const texts = unit.lines.map((l, k) => doc.lines[l].slice(unit.cols[k]))
  const starts: number[] = []
  let at = 0
  for (const t of texts) {
    starts.push(at)
    at += t.length + 1
  }
  const toAbs = (u: number): number => {
    let k = starts.length - 1
    while (k > 0 && starts[k] > u) k--
    return doc.starts[unit.lines[k]] + unit.cols[k] + (u - starts[k])
  }
  const touch = (from: number, to: number): boolean => sel !== null && toAbs(from) <= sel.hi && toAbs(to) >= sel.lo

  const out: Part[][] = texts.map(() => [])
  let cur = 0
  let pos = 0
  const emit = (part: Part): void => {
    part.text.split('\n').forEach((seg, i) => {
      if (i > 0) cur++
      if (seg) out[cur].push({ ...part, text: seg })
    })
  }

  const walk = (nodes: readonly Inline[], cls: string, href?: string): void => {
    for (const n of nodes) {
      const from = pos
      switch (n.kind) {
        case 'text':
          emit({ text: n.text, cls, hidden: false, href })
          pos += n.text.length
          break
        case 'esc':
          emit(mark('\\', touch(from, from + 2)))
          emit({ text: n.char, cls, hidden: false, href })
          pos += 2
          break
        case 'code': {
          const on = touch(from, from + n.open.length + n.text.length + n.close.length)
          emit(mark(n.open, on))
          emit({ text: n.text, cls: `${cls} md-ic`.trim(), hidden: false, href })
          emit(mark(n.close, on))
          pos += n.open.length + n.text.length + n.close.length
          break
        }
        case 'image': {
          const len = n.open.length + n.alt.length + n.close.length
          emit(mark(n.open + n.alt + n.close, touch(from, from + len)))
          out[cur].push({ text: '', cls: '', hidden: false, img: { src: n.src, alt: n.alt } })
          pos += len
          break
        }
        case 'span':
        case 'link': {
          const on = touch(from, from + n.open.length + inlineSource(n.children).length + n.close.length)
          const inner = n.kind === 'span' ? `${cls} ${SPAN_CLASS[n.tag]}`.trim() : `${cls} md-a`.trim()
          if (n.open) emit(mark(n.open, on))
          pos += n.open.length
          walk(n.children, inner, n.kind === 'link' ? n.href : href)
          if (n.close) emit(mark(n.close, on))
          pos += n.close.length
          break
        }
      }
    }
  }
  walk(parseInline(texts.join('\n')), '')
  return out
}

/** 相邻且样式相同的片段并成一个，少生成 DOM 节点 */
function merge(parts: Part[]): Part[] {
  const out: Part[] = []
  for (const p of parts) {
    const last = out[out.length - 1]
    if (last && !last.img && !p.img && last.cls === p.cls && last.hidden === p.hidden && last.href === p.href) last.text += p.text
    else out.push({ ...p })
  }
  return out
}

/** 片段转 HTML；没有样式的直接输出文字。整行都看不见时补一个 <br>，否则这一行没有高度、光标也放不进去 */
function body(parts: Part[]): string {
  const html = merge(parts)
    .map((p) => {
      if (p.img) return `<img class="md-img" src="${esc(p.img.src)}" alt="${esc(p.img.alt)}" draggable="false" contenteditable="false">`
      if (!p.cls) return esc(p.text)
      return `<span class="${p.cls}"${p.href ? ` data-href="${esc(p.href)}"` : ''}>${esc(p.text)}</span>`
    })
    .join('')
  return parts.some((p) => !p.hidden && (p.text || p.img)) ? html : `${html}<br>`
}

/** 拼出一行的 div */
function row(cls: string, attrs: string, parts: Part[]): string {
  return `<div class="md-line ${cls}"${attrs}>${body(parts)}</div>`
}

/** 列表项的缩进层数和编号：照只读渲染器的块树来数，保证编辑时和保存后的层级一致 */
function listLayout(value: string): { depth: Map<number, number>; ordinal: Map<number, number> } {
  const depth = new Map<number, number>()
  const ordinal = new Map<number, number>()
  const walk = (blocks: readonly Block[], d: number): void => {
    for (const b of blocks) {
      if (b.kind !== 'list') continue
      b.items.forEach((item, idx) => {
        depth.set(item.line, d)
        if (b.ordered) ordinal.set(item.line, b.start + idx)
        walk(item.children, d + 1)
      })
    }
  }
  walk(parseBlocks(value), 0)
  return { depth, ordinal }
}

/** 围栏代码块：起止行号，是否有代码行，选区是否碰到 */
interface Fence {
  open: number
  close: number
  hasCode: boolean
  active: boolean
}

/** 找出所有围栏代码块（没闭合的算到文末） */
function findFences(doc: Doc, first: number, last: number, hasSel: boolean): Fence[] {
  const out: Fence[] = []
  const n = doc.lines.length
  for (let i = 0; i < n; i++) {
    if (doc.infos[i].kind !== 'fence') continue
    let j = i + 1
    while (j < n && doc.infos[j].kind === 'code') j++
    const close = j < n && doc.infos[j].kind === 'fence' ? j : j - 1
    out.push({ open: i, close, hasCode: j > i + 1, active: hasSel && i <= last && close >= first })
    i = close
  }
  return out
}

/**
 * 把全文画成逐行的 HTML（下标就是行号）。
 * sel 是当前选区；选区碰到的行和行内格式，记号会亮出来。
 */
export function renderLines(value: string, sel: Span | null): string[] {
  const doc = analyze(value)
  const n = doc.lines.length
  const first = sel ? lineIndexAt(doc, sel.lo) : -1
  const last = sel ? lineIndexAt(doc, sel.hi) : -1
  const touched = (i: number): boolean => i >= first && i <= last
  const { depth, ordinal } = listLayout(value)
  const fences = findFences(doc, first, last, sel !== null)
  const fenceOf = new Map<number, Fence>()
  for (const f of fences) for (let i = f.open; i <= f.close; i++) fenceOf.set(i, f)

  const html: string[] = new Array<string>(n)
  const plain = (i: number): Part[] => [{ text: doc.lines[i], cls: '', hidden: false }]

  for (let i = 0; i < n; ) {
    const info = doc.infos[i]
    let next = i + 1
    switch (info.kind) {
      case 'blank':
        html[i] = row('md-blank', '', plain(i))
        break
      case 'code': {
        const f = fenceOf.get(i) as Fence
        const edge = `${i === f.open + 1 ? ' md-cf' : ''}${i === f.close || (i === f.close - 1 && doc.infos[f.close].kind === 'fence') ? ' md-cl' : ''}`
        html[i] = row(`md-code${edge}`, '', plain(i))
        break
      }
      case 'fence': {
        const f = fenceOf.get(i) as Fence
        const off = f.hasCode && !f.active
        html[i] = row(`md-fence${off ? ' md-off' : ''}`, '', [mark(doc.lines[i], !off)])
        break
      }
      case 'hr':
        html[i] = row('md-hr', '', [mark(doc.lines[i], touched(i))])
        break
      case 'heading':
      case 'quote': {
        const lead = mark(doc.lines[i].slice(0, info.markLen), touched(i))
        const rest = renderUnit(doc, { lines: [i], cols: [info.markLen] }, sel)[0]
        const attrs = info.kind === 'heading' ? '' : ` data-q="${Math.min(info.level ?? 1, MAX_QUOTE_BARS)}"`
        const cls = info.kind === 'heading' ? `md-h md-h${info.level}` : 'md-quote'
        html[i] = row(cls, attrs, [lead, ...rest])
        break
      }
      case 'item': {
        const lines = [i]
        while (next < n && doc.owner[next] === i) lines.push(next++)
        const cols = lines.map((l) => (l === i ? info.markLen : hiddenLen(doc, l)))
        const parts = renderUnit(doc, { lines, cols }, sel)
        const d = depth.get(i) ?? 0
        lines.forEach((l, k) => {
          const zone: Part = { text: doc.lines[l].slice(0, cols[k]), cls: 'md-hide', hidden: true }
          const lead = zone.text ? [zone] : []
          if (l !== i) {
            html[l] = row('md-cont', ` data-d="${d}"`, [...lead, ...parts[k]])
            return
          }
          const task = info.checked === undefined ? '' : ` data-task="${info.checked ? 1 : 0}"`
          const bullet = info.ordered ? `${ordinal.get(i) ?? info.number}.` : BULLETS[d % BULLETS.length]
          html[l] = row('md-item', ` data-d="${d}" data-b="${esc(bullet)}"${task}`, [...lead, ...parts[k]])
        })
        break
      }
      case 'para': {
        const lines = [i]
        while (next < n && doc.infos[next].kind === 'para' && doc.owner[next] < 0) lines.push(next++)
        const parts = renderUnit(doc, { lines, cols: lines.map(() => 0) }, sel)
        lines.forEach((l, k) => (html[l] = row('md-p', '', parts[k])))
        break
      }
    }
    i = next
  }
  return html
}
