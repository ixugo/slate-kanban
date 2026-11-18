// 行内 Markdown 解析：把一段文字拆成带源码记号的节点树。
// 编辑器（边输入边渲染）和只读渲染器共用这一份，所以"输入时看到的"和"保存后看到的"始终一致。
// 每个节点都保留自己的记号（open / close），把树还原成字符串正好等于原文；编辑器靠它把记号藏起来或亮出来。
// 只认一小撮安全语法：粗体、斜体、删除线、<u> 下划线、行内代码、链接（http、https、mailto）、裸网址和反斜杠转义。
// 其它 HTML 一律当文字，图片语法不产生图片。强调的配对规则照 CommonMark，并对中文标点相邻的情况放宽。

export type SpanTag = 'strong' | 'em' | 'del' | 'u'

export type Inline =
  | { kind: 'text'; text: string }
  /** 反斜杠转义：源码是 `\` 加 char，显示时只剩 char */
  | { kind: 'esc'; char: string }
  /** 行内代码：open / close 是同样长度的反引号串，text 是中间的原文 */
  | { kind: 'code'; open: string; close: string; text: string }
  /** 粗体、斜体、删除线、下划线：open / close 是两头的记号 */
  | { kind: 'span'; tag: SpanTag; open: string; close: string; children: Inline[] }
  /** 链接：写法 [文字](地址) 时 open 是 `[`、close 是 `](地址)`；裸网址时两者都是空串 */
  | { kind: 'link'; open: string; close: string; href: string; children: Inline[] }

/** 一段文字里最多拿多少个分隔符段去配对强调；超出的都当普通文字，防止病态输入把配对拖成平方级 */
const MAX_DELIMS = 2000
/** 链接文字、链接地址的最大长度；更长的不当链接，同样是为了限制扫描量 */
const MAX_LABEL = 1000
const MAX_HREF = 2000

const WHITESPACE = /\s/
const PUNCTUATION = /[\p{P}\p{S}]/u
const CJK = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u
const ASCII_PUNCTUATION = /[!-/:-@[-`{-~]/
/** 裸网址：到空白、尖括号、中日韩文字和全角标点为止，避免把后面紧跟的中文吞进地址 */
const BARE_URL = /https?:\/\/[^\s<>\u3000-\u303f\uff00-\uffef\u3040-\u30ff\u3400-\u4dbf\u4e00-\u9fff\uac00-\ud7af]{1,2000}/y
/** 网址末尾这些符号多半是句子的标点或下一个记号，不算网址 */
const URL_TRAILING = /[.,;:!?'"*_~]/

/** 连续分隔符段（若干个 * 或 _，或恰好两个 ~）：配对强调时用 */
interface Delim {
  ch: string
  len: number
  /** 最初的长度，"三的倍数"规则要用 */
  orig: number
  canOpen: boolean
  canClose: boolean
}

type DelimToken = { t: 'delim'; d: Delim }
type Token = { t: 'node'; node: Inline } | DelimToken

/** 判断链接协议是否允许：只放行 http、https、mailto */
export function isSafeHref(href: string): boolean {
  return /^(https?:\/\/|mailto:)/i.test(href.trim())
}

/** 取 i 之前的一个字符（按码点），字符串开头当作空白 */
function charBefore(src: string, i: number): string {
  if (i <= 0) return ' '
  const low = src.charCodeAt(i - 1)
  if (i >= 2 && low >= 0xdc00 && low <= 0xdfff) {
    const high = src.charCodeAt(i - 2)
    if (high >= 0xd800 && high <= 0xdbff) return src.slice(i - 2, i)
  }
  return src[i - 1]
}

/** 取 i 处的一个字符（按码点），字符串结尾当作空白 */
function charAfter(src: string, i: number): string {
  const cp = i < src.length ? src.codePointAt(i) : undefined
  return cp === undefined ? ' ' : String.fromCodePoint(cp)
}

/** 从 i 开始，连续出现 ch 的个数 */
function runLength(src: string, i: number, ch: string): number {
  let n = 0
  while (src[i + n] === ch) n++
  return n
}

/** 找行内代码的结束：从 from 往后找长度恰好为 run 的反引号串，找不到返回 -1 */
function findCodeEnd(src: string, from: number, run: number): number {
  let j = src.indexOf('`', from)
  while (j >= 0) {
    const n = runLength(src, j, '`')
    if (n === run) return j
    j = src.indexOf('`', j + n)
  }
  return -1
}

/**
 * 判断一段分隔符能不能当开头、能不能当结尾（CommonMark 的"左右夹持"规则）。
 * 中日韩文字和空白、标点一样对待，这样「**注意：**请看」这类写法里的粗体也能生效。
 * 下划线额外禁止出现在单词中间，所以 snake_case_name 不会变成斜体。
 */
function flanking(src: string, start: number, end: number, ch: string): { canOpen: boolean; canClose: boolean } {
  const before = charBefore(src, start)
  const after = charAfter(src, end)
  const wsBefore = WHITESPACE.test(before)
  const wsAfter = WHITESPACE.test(after)
  const puBefore = PUNCTUATION.test(before)
  const puAfter = PUNCTUATION.test(after)
  const left = !wsAfter && (!puAfter || wsBefore || puBefore || CJK.test(before))
  const right = !wsBefore && (!puBefore || wsAfter || puAfter || CJK.test(after))
  if (ch === '_') return { canOpen: left && (!right || puBefore), canClose: right && (!left || puAfter) }
  return { canOpen: left, canClose: right }
}

/** 从 start 处的 `[` 起尝试解析 [文字](地址)；成功返回节点和结束位置，不是链接返回 null */
function tryLink(src: string, start: number): { node: Inline; end: number } | null {
  let depth = 0
  let j = start
  for (; j < src.length && j - start <= MAX_LABEL; j++) {
    const c = src[j]
    if (c === '\\') {
      j++
    } else if (c === '`') {
      const run = runLength(src, j, '`')
      const end = findCodeEnd(src, j + run, run)
      j = end < 0 ? j + run - 1 : end + run - 1
    } else if (c === '[') {
      depth++
    } else if (c === ']' && --depth === 0) {
      break
    }
  }
  if (j >= src.length || depth !== 0 || src[j + 1] !== '(' || j === start + 1) return null

  let k = j + 2
  let parens = 0
  for (; k < src.length && k - j <= MAX_HREF; k++) {
    const c = src[k]
    if (WHITESPACE.test(c)) return null
    if (c === '(') parens++
    else if (c === ')' && parens-- === 0) break
  }
  if (src[k] !== ')') return null

  const href = src.slice(j + 2, k)
  const whole = src.slice(start, k + 1)
  // 不安全的协议不做成链接，原样显示成文字
  if (!isSafeHref(href)) return { node: { kind: 'text', text: whole }, end: k + 1 }
  const children = parseInline(src.slice(start + 1, j), false)
  return { node: { kind: 'link', open: '[', close: `](${href})`, href, children }, end: k + 1 }
}

/** 在 i 处匹配裸网址，去掉末尾的标点和多余的右括号；不是网址返回 null */
function matchBareUrl(src: string, i: number): string | null {
  BARE_URL.lastIndex = i
  const m = BARE_URL.exec(src)
  if (!m) return null
  let url = m[0]
  for (;;) {
    const last = url[url.length - 1]
    const unbalanced = last === ')' && url.split('(').length < url.split(')').length
    if (!URL_TRAILING.test(last) && !unbalanced) break
    url = url.slice(0, -1)
  }
  return /^https?:\/\/./i.test(url) ? url : null
}

/** 把源码切成记号序列：普通节点原样放好，分隔符段留给后面配对 */
function tokenize(src: string, allowLinks: boolean): Token[] {
  const out: Token[] = []
  let text = ''
  const flush = () => {
    if (text) out.push({ t: 'node', node: { kind: 'text', text } })
    text = ''
  }
  const push = (node: Inline) => {
    flush()
    out.push({ t: 'node', node })
  }
  // 已经确认"后面没有同样长度的结束串"的反引号长度，避免对同一种长度反复扫到结尾
  const noCodeEnd = new Set<number>()
  let noUnderlineEnd = false

  let i = 0
  while (i < src.length) {
    const ch = src[i]
    if (ch === '\\' && i + 1 < src.length && ASCII_PUNCTUATION.test(src[i + 1])) {
      push({ kind: 'esc', char: src[i + 1] })
      i += 2
    } else if (ch === '`') {
      const run = runLength(src, i, '`')
      const end = noCodeEnd.has(run) ? -1 : findCodeEnd(src, i + run, run)
      if (end < 0) {
        noCodeEnd.add(run)
        text += src.slice(i, i + run)
        i += run
      } else {
        push({ kind: 'code', open: '`'.repeat(run), close: '`'.repeat(run), text: src.slice(i + run, end) })
        i = end + run
      }
    } else if (ch === '[' && allowLinks && charBefore(src, i) !== '!') {
      const link = tryLink(src, i)
      if (link) {
        push(link.node)
        i = link.end
      } else {
        text += ch
        i++
      }
    } else if (ch === '<' && src.startsWith('<u>', i) && !noUnderlineEnd) {
      const close = src.indexOf('</u>', i + 3)
      if (close > i + 3) {
        push({ kind: 'span', tag: 'u', open: '<u>', close: '</u>', children: parseInline(src.slice(i + 3, close), allowLinks) })
        i = close + 4
      } else {
        if (close < 0) noUnderlineEnd = true
        text += ch
        i++
      }
    } else if (ch === 'h' && allowLinks && !/\w/.test(charBefore(src, i)) && matchBareUrl(src, i)) {
      const url = matchBareUrl(src, i) as string
      push({ kind: 'link', open: '', close: '', href: url, children: [{ kind: 'text', text: url }] })
      i += url.length
    } else if (ch === '*' || ch === '_' || ch === '~') {
      const run = runLength(src, i, ch)
      const f = ch === '~' && run !== 2 ? { canOpen: false, canClose: false } : flanking(src, i, i + run, ch)
      if (f.canOpen || f.canClose) {
        flush()
        out.push({ t: 'delim', d: { ch, len: run, orig: run, ...f } })
      } else {
        text += src.slice(i, i + run)
      }
      i += run
    } else {
      text += ch
      i++
    }
  }
  flush()
  return out
}

/** 把记号转回节点：没配上对的分隔符变回普通文字，相邻的文字合并 */
function toNodes(tokens: readonly Token[]): Inline[] {
  const out: Inline[] = []
  for (const tk of tokens) {
    const node: Inline = tk.t === 'node' ? tk.node : { kind: 'text', text: tk.d.ch.repeat(tk.d.len) }
    const last = out[out.length - 1]
    if (node.kind === 'text' && last?.kind === 'text') out[out.length - 1] = { kind: 'text', text: last.text + node.text }
    else out.push(node)
  }
  return out
}

/** 判断开头段和结尾段能不能配成一对（含 CommonMark 的"三的倍数"规则） */
function canPair(opener: Delim, closer: Delim): boolean {
  if (opener.ch !== closer.ch || !opener.canOpen) return false
  const either = opener.canClose || closer.canOpen
  const sum = opener.orig + closer.orig
  return !(either && sum % 3 === 0 && !(opener.orig % 3 === 0 && closer.orig % 3 === 0))
}

/**
 * 配对强调：从左到右找每个能当结尾的段，往回找最近的能配的开头，
 * 把中间的记号包成一个节点；两边各用掉 1 个或 2 个符号，剩下的继续参与配对。
 */
function pairEmphasis(tokens: Token[]): void {
  let ci = 0
  while (ci < tokens.length) {
    const closerToken = tokens[ci]
    if (closerToken.t !== 'delim' || !closerToken.d.canClose) {
      ci++
      continue
    }
    const closer = closerToken.d
    let oi = ci - 1
    while (oi >= 0) {
      const t = tokens[oi]
      if (t.t === 'delim' && canPair(t.d, closer)) break
      oi--
    }
    if (oi < 0) {
      ci++
      continue
    }
    const openerToken = tokens[oi] as DelimToken
    const opener = openerToken.d
    const use = closer.ch === '~' ? 2 : opener.len >= 2 && closer.len >= 2 ? 2 : 1
    const tag: SpanTag = closer.ch === '~' ? 'del' : use === 2 ? 'strong' : 'em'
    const mark = closer.ch.repeat(use)
    const node: Inline = { kind: 'span', tag, open: mark, close: mark, children: toNodes(tokens.slice(oi + 1, ci)) }
    opener.len -= use
    closer.len -= use
    const replacement: Token[] = []
    if (opener.len > 0) replacement.push(openerToken)
    replacement.push({ t: 'node', node })
    if (closer.len > 0) replacement.push(closerToken)
    tokens.splice(oi, ci - oi + 1, ...replacement)
    // 节点之后的位置：结尾段若还有剩余，正好在这里，下一轮继续配
    ci = oi + (opener.len > 0 ? 1 : 0) + 1
  }
}

/**
 * 解析一段行内文字。allowLinks 为 false 时不识别链接和裸网址（用于链接文字本身，避免链接套链接）。
 * 返回的节点树满足：inlineSource(parseInline(s)) === s。
 */
export function parseInline(src: string, allowLinks = true): Inline[] {
  const tokens = tokenize(src, allowLinks)
  const delimCount = tokens.reduce((n, t) => n + (t.t === 'delim' ? 1 : 0), 0)
  if (delimCount <= MAX_DELIMS) pairEmphasis(tokens)
  return toNodes(tokens)
}

/** 把节点树还原成源码，测试和编辑器用它核对"不丢字、不多字" */
export function inlineSource(nodes: readonly Inline[]): string {
  return nodes
    .map((n) => {
      switch (n.kind) {
        case 'text':
          return n.text
        case 'esc':
          return `\\${n.char}`
        case 'code':
          return n.open + n.text + n.close
        default:
          return n.open + inlineSource(n.children) + n.close
      }
    })
    .join('')
}
