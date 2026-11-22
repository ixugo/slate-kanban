// 活动抽屉：从右侧滑出，按时间从新到旧列出看板上谁做了什么，可按事件类型和操作者筛选。
// 懒加载，第一次点「活动」按钮时才下载。

import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { useTranslation } from '@/i18n/react'
import type { CardView } from '@/lib/api/types'
import { ActivityPanel } from './ActivityPanel'

interface ActivityDrawerProps {
  boardId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  /** 看板上现有的任务 */
  cardsById: ReadonlyMap<number, CardView>
}

/** 看板的活动抽屉 */
export default function ActivityDrawer({ boardId, open, onOpenChange, cardsById }: ActivityDrawerProps) {
  const { t } = useTranslation()
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="gap-0 data-[side=right]:w-full">
        <SheetHeader className="border-b pr-14">
          <SheetTitle>{t('activity.title')}</SheetTitle>
          <SheetDescription>{t('activity.description')}</SheetDescription>
        </SheetHeader>
        <ActivityPanel boardId={boardId} cardsById={cardsById} onOpenCard={() => onOpenChange(false)} />
      </SheetContent>
    </Sheet>
  )
}
