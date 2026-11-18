// 项目列表页：一眼看到所有项目，进入某个项目，或者新建一个。

import { PlusIcon } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { Link } from 'react-router-dom'
import { Logo } from '@/app/Logo'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import type { Board } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { formatRelative } from '@/lib/format'
import { useLanguage } from '@/i18n/useLanguage'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
/** 新建项目弹窗体积大，点「新建」后再加载，减轻首屏 */
const CreateBoardDialog = lazy(() =>
  import('./CreateBoardDialog').then((m) => ({ default: m.CreateBoardDialog })),
)
import { useBoards } from './useBoards'

/** 列表里的一张项目卡片：编号前缀、名称、说明、最近更新时间；agent 回填过项目路径时才显示路径 */
function BoardCard({ board }: { board: Board }) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  return (
    <Link
      to={`/b/${board.id}`}
      className="group flex min-h-36 flex-col gap-2.5 rounded-[1.25rem] bg-card p-5 shadow-card transition-[box-shadow,translate] duration-300 ease-spring outline-none hover:-translate-y-0.5 hover:shadow-card-hover focus-visible:ring-[3px] focus-visible:ring-ring/50 active:translate-y-0 active:duration-100"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="rounded-full bg-secondary px-2 py-0.5 font-mono text-[0.6875rem] font-medium tracking-wider text-muted-foreground">
          {board.slug}
        </span>
        <span className="text-xs text-muted-foreground">
          {t('boards.updated', { time: formatRelative(board.updated_at, lang) })}
        </span>
      </div>
      <h2 className="truncate text-[1.0625rem] leading-6 font-semibold tracking-tight">{board.name}</h2>
      {board.description && <p className="line-clamp-2 text-ui text-muted-foreground">{board.description}</p>}
    </Link>
  )
}

/** 一个都没有时的引导：说明项目是做什么的，并给出创建入口 */
function EmptyState({ onCreate }: { onCreate: () => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center gap-5 rounded-[1.5rem] bg-card p-12 text-center shadow-card">
      <Logo className="size-14" />
      <div className="space-y-1.5">
        <h2 className="text-title font-semibold tracking-tight">{t('boards.empty.title')}</h2>
        <p className="max-w-md text-ui text-muted-foreground">{t('boards.empty.body')}</p>
      </div>
      <Button size="lg" onClick={onCreate}>
        <PlusIcon />
        {t('boards.empty.cta')}
      </Button>
    </div>
  )
}

/** 项目列表页组件 */
export function BoardListPage() {
  const { t } = useTranslation()
  const { data: boards, error, isPending, refetch } = useBoards()
  const [creating, setCreating] = useState(false)
  useDocumentTitle(t('boards.title'))

  return (
    <div className="mx-auto w-full max-w-5xl space-y-6 px-5 py-8 sm:px-8 sm:py-10">
      <div className="flex items-end justify-between gap-3">
        <div className="space-y-1.5">
          <h1 className="text-display font-semibold tracking-tight">{t('boards.title')}</h1>
          <p className="text-body text-muted-foreground">{t('boards.subtitle')}</p>
        </div>
        <Button size="lg" onClick={() => setCreating(true)}>
          <PlusIcon />
          {t('boards.new')}
        </Button>
      </div>

      {isPending && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-busy="true">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-36 rounded-[1.25rem]" />
          ))}
        </div>
      )}

      {error && (
        <div role="alert" className="flex items-center justify-between gap-3 rounded-2xl bg-destructive/10 p-4">
          <div className="space-y-0.5">
            <p className="font-medium text-destructive">{t('boards.loadFailed')}</p>
            <p className="text-muted-foreground">{describeError(error, t).message}</p>
          </div>
          <Button variant="outline" onClick={() => void refetch()}>
            {t('common.retry')}
          </Button>
        </div>
      )}

      {boards && boards.length === 0 && <EmptyState onCreate={() => setCreating(true)} />}
      {boards && boards.length > 0 && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {boards.map((board) => (
            <BoardCard key={board.id} board={board} />
          ))}
        </div>
      )}

      {creating ? (
        <Suspense fallback={null}>
          <CreateBoardDialog open={creating} onOpenChange={setCreating} />
        </Suspense>
      ) : null}
    </div>
  )
}
