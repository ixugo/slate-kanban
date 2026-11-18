// 看板的标签库提供者：看板页把快照里的标签库和卡片上用过的标签名放进来，磁贴、列表和卡片弹窗按名称查颜色。

import { useMemo, type ReactNode } from 'react'
import type { Label } from '@/lib/api/types'
import { LabelsContext, buildOptions } from './useLabels'

interface LabelsProviderProps {
  boardId: number
  labels: Label[]
  /** 看板上所有卡片的标签，用来列出没登记进库的标签 */
  usedTags: string[]
  children: ReactNode
}

/** 在看板页最外层提供标签库 */
export function LabelsProvider({ boardId, labels, usedTags, children }: LabelsProviderProps) {
  const value = useMemo(() => ({ boardId, labels, options: buildOptions(labels, usedTags) }), [boardId, labels, usedTags])
  return <LabelsContext.Provider value={value}>{children}</LabelsContext.Provider>
}
