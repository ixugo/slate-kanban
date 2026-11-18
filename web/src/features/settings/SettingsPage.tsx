// 看板设置页：常规、列、自动化三个页签。

import { ArrowLeftIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useBoardSnapshot, useBoardView } from '@/features/board/useBoard'
import { ApiError } from '@/lib/api/client'
import { describeError } from '@/lib/errors'
import { parseId } from '@/lib/route'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { AutomationSection } from './AutomationSection'
import { ColumnsSection } from './ColumnsSection'
import { GeneralSection } from './GeneralSection'

/** 看板设置页默认导出 */
export default function SettingsPage() {
  const { t } = useTranslation()
  const { boardId: raw } = useParams()
  const boardId = parseId(raw)
  const { data, error, isPending, refetch, isError } = useBoardSnapshot(boardId)
  const view = useBoardView(data)
  useDocumentTitle(data ? t('settings.title', { name: data.name }) : t('settings.titleFallback'))

  if (boardId === null) {
    return <SettingsMessage title={t('settings.badId.title')} body={t('settings.badId.body')} />
  }

  if (isPending) {
    return (
      <PageShell title={<Skeleton className="h-6 w-48" />}>
        <Skeleton className="h-64 w-full" />
      </PageShell>
    )
  }

  if (isError || !data) {
    const detail = error ? describeError(error, t) : null
    const isGone = error instanceof ApiError && error.reason === 'ErrBoardNotFound'
    return (
      <SettingsMessage
        title={isGone ? t('board.gone.title') : t('settings.loadFailed')}
        body={detail?.message ?? t('errors.ErrNotFound')}
        action={
          <Button type="button" variant="outline" onClick={() => void refetch()}>
            {t('common.retry')}
          </Button>
        }
      />
    )
  }

  return (
    <PageShell
      title={data.name}
      back={
        <Button variant="ghost" size="sm" asChild>
          <Link to={`/b/${boardId}`}>
            <ArrowLeftIcon />
            {t('settings.backToBoard')}
          </Link>
        </Button>
      }
    >
      <Tabs defaultValue="general" className="space-y-6">
        <TabsList className="flex-wrap">
          <TabsTrigger value="general">{t('settings.tabs.general')}</TabsTrigger>
          <TabsTrigger value="columns">{t('settings.tabs.columns')}</TabsTrigger>
          <TabsTrigger value="automation">{t('settings.tabs.automation')}</TabsTrigger>
        </TabsList>
        <TabsContent value="general">
          <GeneralSection board={data} />
        </TabsContent>
        <TabsContent value="columns">
          <ColumnsSection boardId={boardId} columns={view.columns} cards={data.cards} />
        </TabsContent>
        <TabsContent value="automation">
          <AutomationSection boardId={boardId} columns={view.columns} rules={data.rules} />
        </TabsContent>
      </Tabs>
    </PageShell>
  )
}

/** 页面外框：标题与返回链接 */
function PageShell({ title, back, children }: { title: ReactNode; back?: ReactNode; children?: ReactNode }) {
  return (
    <div className="mx-auto w-full max-w-3xl space-y-6 px-5 py-8 sm:px-8 sm:py-10">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-display font-semibold tracking-tight">{title}</h1>
        {back}
      </header>
      {children}
    </div>
  )
}

/** 错误或非法编号时的友好说明 */
function SettingsMessage({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return (
    <PageShell title={title}>
      <p className="text-muted-foreground">{body}</p>
      {action}
    </PageShell>
  )
}