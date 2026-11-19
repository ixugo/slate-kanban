// 看板标签库的上下文和读取方法；提供者在 LabelsContext.tsx。

import { createContext, useContext } from 'react'
import type { Tag } from '@/lib/api/types'

export interface LabelsValue {
  boardId: number
  /** 标签库，按显示顺序 */
  tags: Tag[]
}

/** 不在看板页里时用的空库 */
export const EMPTY_LABELS: LabelsValue = { boardId: 0, tags: [] }

export const LabelsContext = createContext<LabelsValue>(EMPTY_LABELS)

/** 读取当前看板的标签库；不在看板页里时是空库 */
export function useLabels(): LabelsValue {
  return useContext(LabelsContext)
}

/** 按主键查标签；库里没有（比如刚被别人删掉、快照还没刷新）返回 undefined */
export function useTag(id: number): Tag | undefined {
  const { tags } = useLabels()
  return tags.find((tag) => tag.id === id)
}
