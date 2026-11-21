// 活动句子后面的「，查看变更」：标点和链接文字都跟着界面语言走，链接带下划线。
// 点击后才加载对比弹窗（单独的一块代码，连同 diff 算法一起按需下载），再请求这次修改的前后两版。

import { lazy, Suspense, useState } from 'react'
import { useTranslation } from '@/i18n/react'

const DescriptionChangeDialog = lazy(() => import('./DescriptionChangeDialog'))

interface ViewChangeLinkProps {
  /** 被修改的卡片 */
  cardId: number
  /** 这次修改在变更日志里的记录编号 */
  changeId: number
}

/** 句子和链接之间的标点，加上带下划线的「查看变更」按钮；弹窗只在点开后才挂载 */
export function ViewChangeLink({ cardId, changeId }: ViewChangeLinkProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <>
      <span className="text-muted-foreground">{t('activity.change.separator')}</span>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="cursor-pointer rounded-sm text-foreground underline underline-offset-2 focus-visible:ring-[3px] focus-visible:ring-ring/40 focus-visible:outline-none"
      >
        {t('activity.change.link')}
      </button>
      {open && (
        <Suspense fallback={null}>
          <DescriptionChangeDialog cardId={cardId} changeId={changeId} onClose={() => setOpen(false)} />
        </Suspense>
      )}
    </>
  )
}
