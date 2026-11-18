// Agent 接入页：安装 MCP、skill，并按宿主展示命令片段。

import { CopyIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { describeError } from '@/lib/errors'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { copyWithToast } from './copyText'
import { SnippetCard } from './SnippetCard'
import { useConnectGuide } from './useConnectGuide'

/** 接入页默认导出 */
export default function AgentsPage() {
  const { t } = useTranslation()
  const { data, error, isPending, isError, refetch } = useConnectGuide()
  useDocumentTitle(t('agents.title'))

  if (isPending) {
    return (
      <Page>
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40 w-full" />
      </Page>
    )
  }

  if (isError || !data) {
    const msg = error ? describeError(error, t).message : t('errors.ErrNotFound')
    return (
      <Page>
        <h1 className="text-lg font-semibold">{t('agents.title')}</h1>
        <p role="alert" className="text-destructive">
          {msg}
        </p>
        <Button type="button" onClick={() => void refetch()}>
          {t('common.retry')}
        </Button>
      </Page>
    )
  }

  const prompt = t('agents.oneLiner', { url: data.install_url })

  return (
    <Page>
      <header className="space-y-1">
        <h1 className="text-lg font-semibold tracking-tight">{t('agents.title')}</h1>
        <p className="text-muted-foreground">{t('agents.subtitle')}</p>
      </header>
      <ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
        <li>{t('agents.steps.mcp')}</li>
        <li>{t('agents.steps.skill')}</li>
        <li>{t('agents.steps.work')}</li>
      </ol>
      <section className="space-y-2 border border-border p-4">
        <h2 className="text-sm font-medium">{t('agents.promptTitle')}</h2>
        <p className="text-sm">{prompt}</p>
        <CopyPromptButton text={prompt} label={t('common.copy')} ok={t('common.copied')} fail={t('agents.copyFailed')} />
      </section>
      <section className="space-y-2 text-sm text-muted-foreground">
        <p>{t('agents.nameHint')}</p>
        <p>{t('agents.pullHint')}</p>
        <p>{t('agents.columnHint')}</p>
        <p>{t('agents.loopHint')}</p>
        <p>{t('agents.waitHint')}</p>
      </section>
      <Tabs defaultValue={data.hosts[0]?.id ?? 'default'}>
        <TabsList className="flex-wrap">
          {data.hosts.map((h) => (
            <TabsTrigger key={h.id} value={h.id}>
              {h.name}
            </TabsTrigger>
          ))}
        </TabsList>
        {data.hosts.map((h) => (
          <TabsContent key={h.id} value={h.id} className="space-y-4">
            {h.mcp
              .filter((s) => s.kind === 'worker')
              .map((s) => (
                <SnippetCard key={`${h.id}-worker`} snippet={s} />
              ))}
            {h.mcp
              .filter((s) => s.kind === 'manager')
              .map((s) => (
                <details key={`${h.id}-manager`} className="space-y-2">
                  <summary className="cursor-pointer text-sm font-medium">{t('agents.managerFold')}</summary>
                  <p className="text-xs text-muted-foreground">{t('agents.managerHint')}</p>
                  <SnippetCard snippet={s} subdued />
                </details>
              ))}
            {h.skill.map((s) => (
              <SnippetCard key={`${h.id}-skill`} snippet={s} />
            ))}
          </TabsContent>
        ))}
      </Tabs>
    </Page>
  )
}

function Page({ children }: { children: ReactNode }) {
  return <div className="mx-auto w-full max-w-3xl space-y-6 p-4 sm:p-6">{children}</div>
}

function CopyPromptButton({ text, label, ok, fail }: { text: string; label: string; ok: string; fail: string }) {
  return (
    <Button type="button" size="sm" variant="outline" onClick={() => void copyWithToast(text, null, ok, fail)}>
      <CopyIcon />
      {label}
    </Button>
  )
}