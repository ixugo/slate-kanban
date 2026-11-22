import type { ReactNode } from 'react'
import { displayKey } from '@/lib/cardKey'

export interface SearchHighlights {
  title: (title: string) => ReactNode
  key: (key: string) => ReactNode
  tag: (name: string, label: string) => ReactNode
}

type Range = { start: number; end: number }

// 与服务端 strings.ToLower 一样逐字符转小写，保留原文字的索引。
function lower(text: string): string {
  return text.replaceAll('İ', 'i').replaceAll('Σ', 'σ').toLowerCase()
}

function matchRanges(terms: string[], exact: string[] = []): (text: string) => Range[] {
  const needles = [...new Set(terms.map(lower))]
  // key: 和 tag: 用服务端的整字段折叠匹配，普通关键词和 title: 用子串匹配。
  const whole = exact.length > 0
    ? new RegExp(`^(?:${exact.map((term) => term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})$`, 'iu')
    : null

  return (text) => {
    if (whole?.test(text)) return [{ start: 0, end: text.length }]
    if (needles.length === 0) return []
    const haystack = lower(text)
    const ranges: Range[] = []
    for (const needle of needles) {
      for (let start = haystack.indexOf(needle); start >= 0; start = haystack.indexOf(needle, start + 1)) {
        ranges.push({ start, end: start + needle.length })
      }
    }
    ranges.sort((a, b) => a.start - b.start)
    const merged: Range[] = []
    for (const range of ranges) {
      const last = merged.at(-1)
      if (last && range.start < last.end) last.end = Math.max(last.end, range.end)
      else merged.push(range)
    }
    return merged
  }
}

function mark(text: string, ranges: Range[]): ReactNode {
  if (ranges.length === 0) return text
  const parts: ReactNode[] = []
  let cursor = 0
  for (const { start, end } of ranges) {
    parts.push(text.slice(cursor, start), (
      <mark key={start} className="rounded-none bg-[#ffff54] text-black">
        {text.slice(start, end)}
      </mark>
    ))
    cursor = end
  }
  parts.push(text.slice(cursor))
  return parts
}

/** 只从服务端已接受的搜索词提取高亮内容。筛选结果仍由服务端决定。 */
export function buildSearchHighlights(keyword: string): SearchHighlights | undefined {
  const plain: string[] = []
  const title: string[] = []
  const keys: string[] = []
  const tags: string[] = []
  // 切词规则沿用 card_filter.go：引号中的空白不断开，只有开头的未加引号减号表示排除。
  for (const [token] of keyword.matchAll(/(?:"[^"]*"|[^\p{White_Space}"])+/gu)) {
    if (token.startsWith('-')) continue
    const text = token.replaceAll('"', '')
    if (!text) continue
    const field = token.startsWith('"') ? null : /^([a-z]+):(.*)$/is.exec(text)
    if (!field) {
      plain.push(text)
      continue
    }
    const [, prefix, value] = field
    if (!value) continue
    if (prefix.toLowerCase() === 'title') title.push(value)
    if (prefix.toLowerCase() === 'key') keys.push(...value.split('|'))
    if (prefix.toLowerCase() === 'tag') tags.push(...value.split('|'))
  }

  if (plain.length + title.length + keys.length + tags.length === 0) return undefined
  const titleRanges = matchRanges([...plain, ...title])
  const keyRanges = matchRanges(plain, keys)
  const tagRanges = matchRanges(plain, tags)

  return {
    title: (text) => mark(text, titleRanges(text)),
    key: (key) => {
      const shown = displayKey(key)
      const ranges = keyRanges(key)
      if (shown === key) return mark(key, ranges)
      // SLATE-126 显示成 #126。只映射能看见的编号，完整编号命中时连同 # 标记。
      const digitsStart = key.length - shown.length + 1
      return mark(shown, ranges.filter(({ end }) => end > digitsStart).map(({ start, end }) => ({
        start: start === 0 && end === key.length ? 0 : Math.max(1, start - digitsStart + 1),
        end: end - digitsStart + 1,
      })))
    },
    tag: (name, label) => {
      const ranges = tagRanges(name)
      if (name === label) return mark(label, ranges)
      if (ranges.some(({ start, end }) => start === 0 && end === name.length)) {
        return mark(label, [{ start: 0, end: label.length }])
      }
      return label
    },
  }
}
