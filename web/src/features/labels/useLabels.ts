// 看板标签库的上下文和读取方法；提供者在 LabelsContext.tsx。

import { createContext, useContext, type CSSProperties } from 'react'
import type { Label } from '@/lib/api/types'
import { labelStyle } from '@/lib/labelColors'

/** 标签选择器里的一项：库里的标签有编号，只在卡片上用过的标签没有 */
export interface LabelOption {
  /** 库里的编号；没登记进标签库的标签是空串 */
  id: string
  name: string
  color: string
}

export interface LabelsValue {
  boardId: number
  /** 标签库 */
  labels: Label[]
  /** 库里的标签在前，后面是卡片上用过但没登记进库的标签（按名称排序） */
  options: LabelOption[]
}

/** 不在看板页里时用的空库 */
export const EMPTY_LABELS: LabelsValue = { boardId: 0, labels: [], options: [] }

export const LabelsContext = createContext<LabelsValue>(EMPTY_LABELS)

/** 合并标签库和卡片上用过的标签，名称不区分大小写去重 */
export function buildOptions(labels: Label[], usedTags: string[]): LabelOption[] {
  const seen = new Set(labels.map((l) => l.name.toLowerCase()))
  const extra: LabelOption[] = []
  for (const name of usedTags) {
    const key = name.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    extra.push({ id: '', name, color: '' })
  }
  extra.sort((a, b) => a.name.localeCompare(b.name))
  return [...labels.map((l) => ({ id: l.id, name: l.name, color: l.color })), ...extra]
}

/** 读取当前看板的标签库；不在看板页里时是空库 */
export function useLabels(): LabelsValue {
  return useContext(LabelsContext)
}

/** 按标签名查颜色样式；库里没有或没配颜色返回 undefined */
export function useLabelStyle(name: string): CSSProperties | undefined {
  const { labels } = useLabels()
  const found = labels.find((l) => l.name.toLowerCase() === name.toLowerCase())
  return found ? labelStyle(found.color) : undefined
}
