// 看板列表页：一眼看到所有看板，进入某块看板，或者新建一块。

import { FolderGit2Icon, PlusIcon } from 'lucide-react'
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
/** 新建看板弹窗体积大，点「新建」后再加载，减轻首屏 */
const CreateBoardDialog = lazy(() =>
  import('./CreateBoardDialog').then((m) => ({ default: m.CreateBoardDialog })),
)
import { useBoards } from './useBoards'

/** 列表里的一张看板卡片：编号前缀、名称、说明、项目路径、最近更新时间 */
function BoardCard({ board }: { board: Board }) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  return (
    <Link
      to={`/b/${board.id}`}
      className="group flex min-h-32 flex-col gap-2 bg-card p-4 ring-1 ring-foreground/10 transition-colors outline-none hover:ring-signal focus-visible:ring-2 focus-visible:ring-ring"
    >
      <div className="flex items-center justify-between gap-2">
        <span className="bg-muted px-1.5 py-0.5 font-mono text-[10px] font-medium tracking-wider text-muted-foreground">
          {board.slug}
        </span>
        <span className="text-[11px] text-muted-foreground">
          {t('boards.updated', { time: formatRelative(board.updated_at, lang) })}
        </span>
      </div>
      <h2 className="truncate text-sm font-semibold">{board.name}</h2>
      {board.description && <p className="line-clamp-2 text-muted-foreground">{board.description}</p>}
      <p className="mt-auto flex items-center gap-1.5 truncate font-mono text-[11px] text-muted-foreground">
        <FolderGit2Icon className="size-3.5 shrink-0" />
        <span className="truncate">{board.project_path || t('boards.noPath')}</span>
      </p>
    </Link>
  )
}

/** 一个都没有时的引导：说明看板是做什么的，并给出创建入口 */
function EmptyState({ onCreate }: { onCreate: () => void }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-col items-center gap-4 border border-dashed p-10 text-center">
      <Logo className="size-12" />
      <div className="space-y-1">
        <h2 className="text-sm font-semibold">{t('boards.empty.title')}</h2>
        <p className="max-w-md text-muted-foreground">{t('boards.empty.body')}</p>
      </div>
      <Button onClick={onCreate}>
        <PlusIcon />
        {t('boards.empty.cta')}
      </Button>
    </div>
  )
}

/** 看板列表页组件 */
export function BoardListPage() {
  const { t } = useTranslation()
  const { data: boards, error, isPending, refetch } = useBoards()
  const [creating, setCreating] = useState(false)
  useDocumentTitle(t('boards.title'))

  return (
    <div className="mx-auto w-full max-w-5xl space-y-5 p-4 sm:p-6">
      <div className="flex items-end justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-lg font-semibold tracking-tight">{t('boards.title')}</h1>
          <p className="text-muted-foreground">{t('boards.subtitle')}</p>
        </div>
        <Button onClick={() => setCreating(true)}>
          <PlusIcon />
          {t('boards.new')}
        </Button>
      </div>

      {isPending && (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-busy="true">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </div>
      )}

      {error && (
        <div role="alert" className="flex items-center justify-between gap-3 border border-destructive/40 p-4">
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
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
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
