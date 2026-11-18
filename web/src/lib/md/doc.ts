// 把全文切成行并逐行判断种类：编辑命令和实时渲染共用，两边对"哪一行是什么"的看法永远一致。

import { classifyLines, type LineInfo } from './blocks'

/** 一份文档按行切开后的样子 */
export interface Doc {
  lines: string[]
  /** 每行第一个字符在全文里的下标 */
  starts: number[]
  infos: LineInfo[]
  /** 列表项的续行（紧跟着列表项、以空白开头的普通行）所属的列表项行号；不是续行时为 -1 */
  owner: number[]
}

/** 按 \n 切行并判断每行的种类；编辑器内部的换行已经统一成 \n */
export function analyze(value: string): Doc {
  const lines = value.split('\n')
  const infos = classifyLines(lines)
  const starts: number[] = []
  const owner: number[] = []
  let at = 0
  let item = -1
  lines.forEach((line, i) => {
    starts.push(at)
    at += line.length + 1
    if (infos[i].kind === 'item') {
      item = i
      owner.push(-1)
    } else if (item >= 0 && infos[i].kind === 'para' && /^[ \t]/.test(line)) {
      owner.push(item)
    } else {
      item = -1
      owner.push(-1)
    }
  })
  return { lines, starts, infos, owner }
}

/** 全文位置 pos 落在第几行（从 0 数）；换行符本身算在它前面那一行的末尾 */
export function lineIndexAt(doc: Doc, pos: number): number {
  let lo = 0
  let hi = doc.starts.length - 1
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1
    if (doc.starts[mid] <= pos) lo = mid
    else hi = mid - 1
  }
  return lo
}

/** 第 i 行开头永远看不见的字符数：列表项的缩进和记号（含任务方框），以及续行的缩进 */
export function hiddenLen(doc: Doc, i: number): number {
  if (doc.infos[i].kind === 'item') return doc.infos[i].markLen
  if (doc.owner[i] >= 0) return /^[ \t]*/.exec(doc.lines[i])![0].length
  return 0
}

/** 第 i 行正文的起点（相对行首）：标题、引用、列表项是记号之后，续行是缩进之后，其余是 0 */
export function contentStart(doc: Doc, i: number): number {
  const { kind, markLen } = doc.infos[i]
  if (kind === 'heading' || kind === 'quote' || kind === 'item') return markLen
  return hiddenLen(doc, i)
}
