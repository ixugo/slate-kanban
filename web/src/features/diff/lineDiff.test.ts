import { describe, expect, it } from 'vitest'
import {
  MAX_INLINE_DIFF_CHARS,
  MAX_LINE_EDIT_DISTANCE,
  diffLines,
  diffStats,
  splitLines,
  tokenizeLine,
  type DiffRow,
} from './lineDiff'

/** 从行列表里取出各 kind 序列，方便断言结构 */
function kinds(rows: DiffRow[]): string[] {
  return rows.map((r) => r.kind)
}

describe('splitLines', () => {
  it('空串返回空数组', () => {
    expect(splitLines('')).toEqual([])
  })

  it('末尾换行产生最后一个空行', () => {
    expect(splitLines('a\n')).toEqual(['a', ''])
  })

  it('CRLF 与单独 LF 都能切', () => {
    expect(splitLines('a\r\nb')).toEqual(['a', 'b'])
    expect(splitLines('a\rb')).toEqual(['a', 'b'])
  })
})

describe('diffLines', () => {
  it('完全相同：全是 equal', () => {
    const rows = diffLines('line1\nline2', 'line1\nline2')
    expect(kinds(rows)).toEqual(['equal', 'equal'])
    expect(rows.every((r) => r.oldNo === r.newNo)).toBe(true)
  })

  it('只新增', () => {
    const rows = diffLines('', 'new')
    expect(rows).toHaveLength(1)
    expect(rows[0].kind).toBe('insert')
    expect(rows[0].oldNo).toBeNull()
    expect(rows[0].newNo).toBe(1)
  })

  it('只删除', () => {
    const rows = diffLines('gone', '')
    expect(rows).toHaveLength(1)
    expect(rows[0].kind).toBe('delete')
    expect(rows[0].newNo).toBeNull()
    expect(rows[0].oldNo).toBe(1)
  })

  it('单行修改：change 且带行内分段', () => {
    const rows = diffLines('hello world', 'hello there')
    const changed = rows.find((r) => r.kind === 'change')
    expect(changed).toBeDefined()
    expect(changed!.oldSegments!.some((s) => s.changed && s.text.includes('world'))).toBe(true)
    expect(changed!.newSegments!.some((s) => s.changed && s.text.includes('there'))).toBe(true)
    expect(changed!.oldSegments!.some((s) => !s.changed && s.text.includes('hello'))).toBe(true)
  })

  it('一对多替换：多出来的行变成纯 delete / insert', () => {
    const rows = diffLines('a\nb\nc', 'x\ny')
    expect(rows.some((r) => r.kind === 'change')).toBe(true)
    expect(rows.some((r) => r.kind === 'delete')).toBe(true)
  })

  it('旧文本为空时全是 insert', () => {
    const rows = diffLines('', 'a\nb')
    expect(kinds(rows)).toEqual(['insert', 'insert'])
  })

  it('新文本为空时全是 delete', () => {
    const rows = diffLines('a\nb', '')
    expect(kinds(rows)).toEqual(['delete', 'delete'])
  })

  it('只有换行符风格不同（CRLF 与 LF）时按行比较没有差异', () => {
    const rows = diffLines('a\r\nb', 'a\nb')
    expect(kinds(rows)).toEqual(['equal', 'equal'])
  })

  it('中文行内差异按字切 token', () => {
    const rows = diffLines('你好世界', '你好宇宙')
    const ch = rows.find((r) => r.kind === 'change')!
    expect(ch.oldSegments!.filter((s) => s.changed).some((s) => s.text.includes('世'))).toBe(true)
    expect(ch.newSegments!.filter((s) => s.changed).some((s) => s.text.includes('宇'))).toBe(true)
  })

  it('两行完全不相关时整行标 changed', () => {
    const rows = diffLines('aaaaaaaaaa', 'zzzzzzzzzz')
    const ch = rows.find((r) => r.kind === 'change')!
    expect(ch.oldSegments).toEqual([{ text: 'aaaaaaaaaa', changed: true }])
    expect(ch.newSegments).toEqual([{ text: 'zzzzzzzzzz', changed: true }])
  })

  it('超长行跳过行内差异', () => {
    const long = 'x'.repeat(MAX_INLINE_DIFF_CHARS + 1)
    const rows = diffLines(long, long + 'y')
    const ch = rows.find((r) => r.kind === 'change')!
    expect(ch.oldSegments).toEqual([{ text: long, changed: true }])
  })

  it('编辑距离超限时退化成整块替换且不崩溃', () => {
    const n = MAX_LINE_EDIT_DISTANCE + 50
    const oldText = Array.from({ length: n }, (_, i) => `old-${i}`).join('\n')
    const newText = Array.from({ length: n }, (_, i) => `new-${i}`).join('\n')
    const rows = diffLines(oldText, newText)
    expect(rows.length).toBeGreaterThan(0)
    expect(rows.every((r) => ['delete', 'insert', 'change'].includes(r.kind))).toBe(true)
  })

  it('大输入性能：3000 行改 5 处应在 500ms 内', () => {
    const lines = Array.from({ length: 3000 }, (_, i) => `line-${i}`)
    const oldText = lines.join('\n')
    const next = [...lines]
    next[10] = 'patched-10'
    next[500] = 'patched-500'
    next[1200] = 'patched-1200'
    next[2500] = 'patched-2500'
    next[2999] = 'patched-2999'
    const t0 = performance.now()
    const rows = diffLines(oldText, next.join('\n'))
    const elapsed = performance.now() - t0
    expect(elapsed).toBeLessThan(500)
    expect(rows.filter((r) => r.kind === 'change').length).toBeGreaterThanOrEqual(5)
  })
})

describe('diffLines 随机输入', () => {
  // 固定种子的伪随机数，保证每次运行结果一致
  function makeRandom(seed: number): () => number {
    let s = seed
    return () => {
      s = (s * 1664525 + 1013904223) % 4294967296
      return s / 4294967296
    }
  }

  // 两个数组的最长公共子序列长度，用来判断差异是不是最短
  function lcsLength(a: string[], b: string[]): number {
    const dp = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0))
    for (let i = 1; i <= a.length; i++) {
      for (let j = 1; j <= b.length; j++) {
        dp[i][j] = a[i - 1] === b[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1])
      }
    }
    return dp[a.length][b.length]
  }

  it('左栏按序还原旧文本、右栏还原新文本，删增行数等于最短编辑距离', () => {
    const random = makeRandom(20260121)
    const alphabet = ['a', 'b', 'c', 'd']
    const randomLines = () =>
      Array.from({ length: Math.floor(random() * 14) }, () => alphabet[Math.floor(random() * alphabet.length)])
    for (let round = 0; round < 300; round++) {
      const oldLines = randomLines()
      const newLines = randomLines()
      const rows = diffLines(oldLines.join('\n'), newLines.join('\n'))
      const left = rows.filter((r) => r.oldNo !== null).map((r) => r.oldText)
      const right = rows.filter((r) => r.newNo !== null).map((r) => r.newText)
      expect(left).toEqual(oldLines)
      expect(right).toEqual(newLines)
      const common = lcsLength(oldLines, newLines)
      expect(diffStats(rows)).toEqual({ added: newLines.length - common, removed: oldLines.length - common })
    }
  })

  it('改动很多的大文本不会占用失控的内存和时间', () => {
    // 2000 行里隔行改动：编辑距离约 2000，会触发上限退化，但必须很快返回且结果仍能还原两侧文本
    const oldLines = Array.from({ length: 2000 }, (_, i) => `row-${i}`)
    const newLines = oldLines.map((line, i) => (i % 2 === 0 ? `changed-${i}` : line))
    const t0 = performance.now()
    const rows = diffLines(oldLines.join('\n'), newLines.join('\n'))
    expect(performance.now() - t0).toBeLessThan(2000)
    expect(rows.filter((r) => r.oldNo !== null).map((r) => r.oldText)).toEqual(oldLines)
    expect(rows.filter((r) => r.newNo !== null).map((r) => r.newText)).toEqual(newLines)
  })
})

describe('diffStats', () => {
  it('计数 insert / delete / change', () => {
    const rows = diffLines('a\nb\nc', 'a\nx\ny')
    const s = diffStats(rows)
    expect(s.added).toBeGreaterThan(0)
    expect(s.removed).toBeGreaterThan(0)
  })

  it('完全相同时 added / removed 都是 0', () => {
    const s = diffStats(diffLines('same', 'same'))
    expect(s).toEqual({ added: 0, removed: 0 })
  })
})

describe('tokenizeLine', () => {
  it('中文每字一个 token', () => {
    expect(tokenizeLine('你好')).toEqual(['你', '好'])
  })
})