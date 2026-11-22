// 描述变更对比弹窗：读取一次描述修改的前后两版，左右对比显示（左边旧版本，右边新版本）。
// 它由活动句子里的「查看变更」按需打开，并且是懒加载的单独一块（见 ViewChangeLink），不增加首屏体积。

import { ChevronDownIcon, ChevronUpIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { DiffView } from '@/features/diff/DiffView'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import { describeError } from '@/lib/errors'
import { formatDateTime } from '@/lib/format'
import { actorLabel } from './describeEvent'
import { useCardChange } from './useCardChange'

interface DescriptionChangeDialogProps {
  cardId: number
  changeId: number
  /** 点关闭按钮、遮罩或按 Esc 时调用，由调用方把弹窗卸掉 */
  onClose: () => void
}

/** 描述变更对比弹窗 */
export default function DescriptionChangeDialog({ cardId, changeId, onClose }: DescriptionChangeDialogProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  // 当前看的是哪一条：初始是点开时那条，用右侧的「前一个 / 后一个」在同一张卡片的修改记录之间翻页
  const [currentId, setCurrentId] = useState(changeId)
  const change = useCardChange(cardId, currentId)
  const record = change.data
  // 副标题写明谁在什么时候改的；还没读到时先显示「加载中」，保证弹窗始终有描述文字
  const subtitle = record
    ? `${actorLabel({ kind: record.actor_kind, name: record.actor }, t)} · ${formatDateTime(record.created_at, lang)}`
    : t('common.loading')

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      {/* 弹窗最高不超过屏幕，内容超出时只有对比区自己滚动，标题和关闭按钮始终可见 */}
      <DialogContent className="flex h-[min(72dvh,42rem)] w-[min(64rem,96vw)] max-w-none flex-col overflow-hidden pr-14 sm:max-w-none">
        {/* 关闭按钮下面的翻页按钮；没有更多修改时禁用 */}
        <div className="absolute top-14 right-4 flex flex-col gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t('activity.change.prev')}
            title={t('activity.change.prev')}
            disabled={!record || record.prev_id === 0}
            onClick={() => record && setCurrentId(record.prev_id)}
          >
            <ChevronUpIcon />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t('activity.change.next')}
            title={t('activity.change.next')}
            disabled={!record || record.next_id === 0}
            onClick={() => record && setCurrentId(record.next_id)}
          >
            <ChevronDownIcon />
          </Button>
        </div>
        <DialogHeader className="shrink-0">
          <DialogTitle>{t('activity.change.title')}</DialogTitle>
          <DialogDescription>{subtitle}</DialogDescription>
        </DialogHeader>
        {change.isPending && <Skeleton aria-busy="true" className="h-48 w-full rounded-md" />}
        {change.isError && (
          <div role="alert" className="flex flex-col items-start gap-3 text-sm">
            <p>{describeError(change.error, t).message}</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void change.refetch()}>
              {t('common.retry')}
            </Button>
          </div>
        )}
        {record && (
          <DiffView
            oldText={record.old_value}
            newText={record.new_value}
            oldTitle={t('activity.change.before')}
            newTitle={t('activity.change.after')}
            identicalText={t('activity.change.identical')}
            className="min-h-0 flex-1"
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
