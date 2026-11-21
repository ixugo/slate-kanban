// 纯文本行级差异：Myers 算法、行内词级高亮、统计。不依赖 React，方便单测和复用。

/** 行级 Myers 差异允许的编辑步数上限；超过就把中间区域整块当「删光再插入」，避免极端输入卡死 */
export const MAX_LINE_EDIT_DISTANCE = 1500

/** 单行超过这个字符数就不做行内词级差异，整行标成改动 */
export const MAX_INLINE_DIFF_CHARS = 2000

/** 两行里「相同 token」的总长度低于较长行长度的这个比例时，视为基本不相关，整行标改 */
export const MIN_RELATED_TOKEN_RATIO = 0.3

export type DiffRowKind = 'equal' | 'delete' | 'insert' | 'change'

/** 行内一段文字，以及这段是否算被改过（给左右两栏分别上色） */
export interface Segment {
  text: string
  changed: boolean
}

/** 并排 diff 里的一行：四种 kind，带左右行号与可选行内分段 */
export interface DiffRow {
  kind: DiffRowKind
  /** 左侧行号，从 1 开始；这一行在旧版不存在时为 null（纯新增、右侧填充行） */
  oldNo: number | null
  /** 右侧行号；新版不存在时为 null */
  newNo: number | null
  oldText: string
  newText: string
  /** 仅 change 行：旧行按改动切成的分段 */
  oldSegments?: Segment[]
  /** 仅 change 行：新行分段 */
  newSegments?: Segment[]
}

type RawOp =
  | { kind: 'equal'; oldIndex: number; newIndex: number }
  | { kind: 'delete'; oldIndex: number }
  | { kind: 'insert'; newIndex: number }

/**
 * 按换行切分字符串，兼容 \r\n、\r、\n。
 * 空串返回 []（无行）；不 trim；末尾换行会多出一个空行，和 VS Code 一致。
 */
export function splitLines(text: string): string[] {
  if (text.length === 0) return []
  return text.split(/\r\n|\r|\n/)
}

/** 判断单个 Unicode 码点是否视作 CJK 字符（行内 diff 时一字一 token） */
function isCjkCodePoint(cp: number): boolean {
  return (
    (cp >= 0x4e00 && cp <= 0x9fff) ||
    (cp >= 0x3400 && cp <= 0x4dbf) ||
    (cp >= 0xf900 && cp <= 0xfaff) ||
    (cp >= 0x3040 && cp <= 0x30ff) ||
    (cp >= 0xac00 && cp <= 0xd7af)
  )
}

/**
 * 把一行切成 token：空白成段、英文按词、标点单独；中文（及日韩）每字一个 token。
 */
export function tokenizeLine(line: string): string[] {
  const tokens: string[] = []
  let i = 0
  while (i < line.length) {
    const cp = line.codePointAt(i)!
    const charLen = cp > 0xffff ? 2 : 1
    if (/\s/.test(line[i])) {
      let j = i + 1
      while (j < line.length && /\s/.test(line[j])) j++
      tokens.push(line.slice(i, j))
      i = j
    } else if (isCjkCodePoint(cp)) {
      tokens.push(line.slice(i, i + charLen))
      i += charLen
    } else {
      let j = i + 1
      while (j < line.length) {
        const c = line.codePointAt(j)!
        if (/\s/.test(line[j]) || isCjkCodePoint(c)) break
        j += c > 0xffff ? 2 : 1
      }
      tokens.push(line.slice(i, j))
      i = j
    }
  }
  return tokens
}

/** 编辑距离超限时：整块视为先删光旧序列再插入新序列 */
function blockReplaceOps(a: string[], b: string[]): RawOp[] {
  const ops: RawOp[] = []
  for (let i = 0; i < a.length; i++) ops.push({ kind: 'delete', oldIndex: i })
  for (let j = 0; j < b.length; j++) ops.push({ kind: 'insert', newIndex: j })
  return ops
}

/**
 * 对两个字符串数组做 Myers O(ND) 差异（迭代实现，不递归）。
 * 每一轮只保存 k ∈ [-d, d] 这一小段的快照，总内存约 D² 个整数，而不是 D × (n+m)，
 * 所以长文本大改时也不会一下子吃掉大量内存。超过 maxD 步时退化成整块替换。
 */
function myersDiff(a: string[], b: string[], maxD: number): RawOp[] {
  const n = a.length
  const m = b.length
  // 一边为空就是整块删或整块插，不用跑算法
  if (n === 0 || m === 0) return blockReplaceOps(a, b)

  const max = Math.min(n + m, maxD)
  const offset = max + 1
  // v[offset + k] 是对角线 k 上目前走到的最远 x；offset 让负的 k 也能当下标
  const v = new Int32Array(2 * max + 3)
  // trace[d] 是第 d 轮结束后 k 从 -d 到 d 的 v 值，回溯时用
  const trace: Int32Array[] = []

  for (let d = 0; d <= max; d++) {
    for (let k = -d; k <= d; k += 2) {
      let x: number
      if (k === -d || (k !== d && v[offset + k - 1] < v[offset + k + 1])) {
        x = v[offset + k + 1]
      } else {
        x = v[offset + k - 1] + 1
      }
      let y = x - k
      while (x < n && y < m && a[x] === b[y]) {
        x++
        y++
      }
      v[offset + k] = x
      if (x >= n && y >= m) {
        trace.push(v.slice(offset - d, offset + d + 1))
        return backtrackMyers(a, b, trace, d)
      }
    }
    trace.push(v.slice(offset - d, offset + d + 1))
  }

  return blockReplaceOps(a, b)
}

/**
 * 从终点 (n, m) 沿各轮快照往回走：每一轮先判断是从上方（插入）还是从左方（删除）走过来的，
 * 再把这一步之后那段斜线（相同的元素）收回来。规则和前进时完全一致，所以能还原出同一条最短路径。
 */
function backtrackMyers(a: string[], b: string[], trace: Int32Array[], d: number): RawOp[] {
  const ops: RawOp[] = []
  let x = a.length
  let y = b.length

  for (let depth = d; depth > 0; depth--) {
    const prev = trace[depth - 1]
    // 第 depth-1 轮的快照里，对角线 k 的下标是 k + depth - 1
    const at = (k: number) => prev[k + depth - 1]
    const k = x - y
    const inserted = k === -depth || (k !== depth && at(k - 1) < at(k + 1))
    const prevK = inserted ? k + 1 : k - 1
    const prevX = at(prevK)
    const prevY = prevX - prevK

    while (x > prevX && y > prevY) {
      x--
      y--
      ops.push({ kind: 'equal', oldIndex: x, newIndex: y })
    }
    if (inserted) {
      y--
      ops.push({ kind: 'insert', newIndex: y })
    } else {
      x--
      ops.push({ kind: 'delete', oldIndex: x })
    }
  }

  // 第 0 轮只有开头那段相同的斜线
  while (x > 0 && y > 0) {
    x--
    y--
    ops.push({ kind: 'equal', oldIndex: x, newIndex: y })
  }

  ops.reverse()
  return ops
}

/**
 * 裁掉首尾相同行，只对中间部分跑 Myers，再拼回带全局下标的操作列表。
 */
function diffLineArrays(oldLines: string[], newLines: string[]): RawOp[] {
  let pre = 0
  const maxPre = Math.min(oldLines.length, newLines.length)
  while (pre < maxPre && oldLines[pre] === newLines[pre]) pre++

  let suf = 0
  const oldRem = oldLines.length - pre
  const newRem = newLines.length - pre
  const maxSuf = Math.min(oldRem, newRem)
  while (suf < maxSuf && oldLines[oldLines.length - 1 - suf] === newLines[newLines.length - 1 - suf]) suf++

  const ops: RawOp[] = []
  for (let i = 0; i < pre; i++) {
    ops.push({ kind: 'equal', oldIndex: i, newIndex: i })
  }

  const midOld = oldLines.slice(pre, oldLines.length - suf)
  const midNew = newLines.slice(pre, newLines.length - suf)
  const midOps = myersDiff(midOld, midNew, MAX_LINE_EDIT_DISTANCE)
  for (const op of midOps) {
    if (op.kind === 'equal') {
      ops.push({ kind: 'equal', oldIndex: op.oldIndex + pre, newIndex: op.newIndex + pre })
    } else if (op.kind === 'delete') {
      ops.push({ kind: 'delete', oldIndex: op.oldIndex + pre })
    } else {
      ops.push({ kind: 'insert', newIndex: op.newIndex + pre })
    }
  }

  const oldTailStart = oldLines.length - suf
  const newTailStart = newLines.length - suf
  for (let i = 0; i < suf; i++) {
    ops.push({
      kind: 'equal',
      oldIndex: oldTailStart + i,
      newIndex: newTailStart + i,
    })
  }

  return ops
}

/**
 * 把连续的 delete 块 + insert 块配成 change：前 min(m,n) 行一一对齐，多出来的行保持纯删/纯增。
 */
function pairDeleteInsertBlocks(ops: RawOp[]): Array<RawOp | { kind: 'change'; oldIndex: number; newIndex: number }> {
  type Mixed = RawOp | { kind: 'change'; oldIndex: number; newIndex: number }
  const out: Mixed[] = []
  let i = 0
  while (i < ops.length) {
    if (ops[i].kind !== 'delete') {
      out.push(ops[i])
      i++
      continue
    }
    const delStart = i
    while (i < ops.length && ops[i].kind === 'delete') i++
    const insStart = i
    while (i < ops.length && ops[i].kind === 'insert') i++
    const dels = ops.slice(delStart, insStart) as Array<{ kind: 'delete'; oldIndex: number }>
    const ins = ops.slice(insStart, i) as Array<{ kind: 'insert'; newIndex: number }>
    if (ins.length === 0) {
      out.push(...dels)
      continue
    }
    const pairs = Math.min(dels.length, ins.length)
    for (let p = 0; p < pairs; p++) {
      out.push({ kind: 'change', oldIndex: dels[p].oldIndex, newIndex: ins[p].newIndex })
    }
    for (let p = pairs; p < dels.length; p++) out.push(dels[p])
    for (let p = pairs; p < ins.length; p++) out.push(ins[p])
  }
  return out
}

/** 把 token 级编辑脚本收成左右两列的 Segment（连续相同 changed 标记的 token 合并） */
function segmentsFromTokenOps(
  oldTokens: string[],
  newTokens: string[],
  ops: RawOp[],
): { oldSegments: Segment[]; newSegments: Segment[] } {
  const oldSegs: Segment[] = []
  const newSegs: Segment[] = []

  // 往一列里追加一段文字：和上一段的改动状态相同就并进去，避免出现一串零碎的小段
  const pushSeg = (list: Segment[], text: string, changed: boolean) => {
    if (text.length === 0) return
    const last = list[list.length - 1]
    if (last && last.changed === changed) last.text += text
    else list.push({ text, changed })
  }

  for (const op of ops) {
    if (op.kind === 'equal') {
      const t = oldTokens[op.oldIndex]
      pushSeg(oldSegs, t, false)
      pushSeg(newSegs, t, false)
    } else if (op.kind === 'delete') {
      pushSeg(oldSegs, oldTokens[op.oldIndex], true)
    } else if (op.kind === 'insert') {
      pushSeg(newSegs, newTokens[op.newIndex], true)
    }
  }

  if (oldSegs.length === 0 && oldTokens.join('').length > 0) {
    oldSegs.push({ text: oldTokens.join(''), changed: true })
  }
  if (newSegs.length === 0 && newTokens.join('').length > 0) {
    newSegs.push({ text: newTokens.join(''), changed: true })
  }

  return { oldSegments: oldSegs, newSegments: newSegs }
}

/**
 * 对一对被配成 change 的行做行内差异；过长或不相关时整行标改。
 */
function inlineDiffLines(oldLine: string, newLine: string): { oldSegments: Segment[]; newSegments: Segment[] } {
  const wholeOld: Segment[] = [{ text: oldLine, changed: true }]
  const wholeNew: Segment[] = [{ text: newLine, changed: true }]

  if (oldLine.length > MAX_INLINE_DIFF_CHARS || newLine.length > MAX_INLINE_DIFF_CHARS) {
    return { oldSegments: wholeOld, newSegments: wholeNew }
  }

  const oldTok = tokenizeLine(oldLine)
  const newTok = tokenizeLine(newLine)
  const tokOps = myersDiff(oldTok, newTok, MAX_LINE_EDIT_DISTANCE)

  let equalLen = 0
  for (const op of tokOps) {
    if (op.kind === 'equal') equalLen += oldTok[op.oldIndex].length
  }
  const maxLen = Math.max(oldLine.length, newLine.length)
  if (maxLen > 0 && equalLen / maxLen < MIN_RELATED_TOKEN_RATIO) {
    return { oldSegments: wholeOld, newSegments: wholeNew }
  }

  return segmentsFromTokenOps(oldTok, newTok, tokOps)
}

/** 把混合操作列表转成带行号的 DiffRow */
function buildRows(
  oldLines: string[],
  newLines: string[],
  mixed: Array<RawOp | { kind: 'change'; oldIndex: number; newIndex: number }>,
): DiffRow[] {
  const rows: DiffRow[] = []
  let oldNo = 0
  let newNo = 0

  for (const op of mixed) {
    if (op.kind === 'equal') {
      oldNo++
      newNo++
      rows.push({
        kind: 'equal',
        oldNo,
        newNo,
        oldText: oldLines[op.oldIndex],
        newText: newLines[op.newIndex],
      })
    } else if (op.kind === 'delete') {
      oldNo++
      rows.push({
        kind: 'delete',
        oldNo,
        newNo: null,
        oldText: oldLines[op.oldIndex],
        newText: '',
      })
    } else if (op.kind === 'insert') {
      newNo++
      rows.push({
        kind: 'insert',
        oldNo: null,
        newNo,
        oldText: '',
        newText: newLines[op.newIndex],
      })
    } else {
      oldNo++
      newNo++
      const oldText = oldLines[op.oldIndex]
      const newText = newLines[op.newIndex]
      const { oldSegments, newSegments } = inlineDiffLines(oldText, newText)
      rows.push({
        kind: 'change',
        oldNo,
        newNo,
        oldText,
        newText,
        oldSegments,
        newSegments,
      })
    }
  }

  return rows
}

/**
 * 比较两段多行文本，返回 VS Code 风格的并排 diff 行。
 * 先做行级 Myers，再把相邻删+增配成 change，最后在 change 行里做词级高亮。
 */
export function diffLines(oldText: string, newText: string): DiffRow[] {
  const oldLines = splitLines(oldText)
  const newLines = splitLines(newText)
  if (oldLines.length === 0 && newLines.length === 0) return []

  const rawOps = diffLineArrays(oldLines, newLines)
  const mixed = pairDeleteInsertBlocks(rawOps)
  return buildRows(oldLines, newLines, mixed)
}

/**
 * 统计新增、删除行数：纯 insert / delete 各算 1；change 同时算删 1 加 1。
 */
export function diffStats(rows: DiffRow[]): { added: number; removed: number } {
  let added = 0
  let removed = 0
  for (const row of rows) {
    if (row.kind === 'insert') added++
    else if (row.kind === 'delete') removed++
    else if (row.kind === 'change') {
      added++
      removed++
    }
  }
  return { added, removed }
}