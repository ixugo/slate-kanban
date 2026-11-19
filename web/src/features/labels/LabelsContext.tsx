// 看板的标签库提供者：看板页把快照里的标签库放进来，磁贴、列表和卡片弹窗按主键查颜色和文字。

import { useMemo, type ReactNode } from 'react'
import type { Tag } from '@/lib/api/types'
import { LabelsContext } from './useLabels'

interface LabelsProviderProps {
  boardId: number
  tags: Tag[]
  children: ReactNode
}

/** 在看板页最外层提供标签库 */
export function LabelsProvider({ boardId, tags, children }: LabelsProviderProps) {
  const value = useMemo(() => ({ boardId, tags }), [boardId, tags])
  return <LabelsContext.Provider value={value}>{children}</LabelsContext.Provider>
}
