// Agent 接入页：一句话提示词、怎么持续监听；各宿主的 MCP、skill（可选）命令片段收在默认折叠的“手动安装”里。

import { CopyIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { ConnectGuide } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { copyWithToast } from './copyText'
import { SnippetCard } from './SnippetCard'
import { useConnectGuide } from './useConnectGuide'
import { WatchSection } from './WatchSection'

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
        <h1 className="text-display font-semibold tracking-tight">{t('agents.title')}</h1>
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
      <header className="space-y-1.5">
        <h1 className="text-display font-semibold tracking-tight">{t('agents.title')}</h1>
        <p className="text-body text-muted-foreground">{t('agents.subtitle')}</p>
      </header>
      <section className="space-y-2.5 rounded-2xl bg-card p-5 shadow-card">
        <h2 className="text-body font-semibold tracking-tight">{t('agents.promptTitle')}</h2>
        <p className="text-ui">{prompt}</p>
        <CopyPromptButton text={prompt} label={t('common.copy')} ok={t('common.copied')} fail={t('agents.copyFailed')} />
      </section>
      <WatchSection guide={data} />
      <details className="space-y-4 rounded-2xl bg-card p-5 shadow-card">
        <summary className="cursor-pointer text-body font-semibold tracking-tight">{t('agents.advanced')}</summary>
        <p className="text-ui text-muted-foreground">{t('agents.advancedHint')}</p>
        <p className="text-ui text-muted-foreground">{t('agents.skillOptional')}</p>
        <p className="text-ui text-muted-foreground">{t('agents.nameHint')}</p>
        <HostTabs guide={data} />
      </details>
    </Page>
  )
}

/** 各 host 的命令片段：MCP、管理视图、skill（可选）和长期挂着的启动恢复命令，放在默认折叠的“手动安装”里 */
function HostTabs({ guide: data }: { guide: ConnectGuide }) {
  const { t } = useTranslation()
  return (
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
                <summary className="cursor-pointer text-ui font-medium">{t('agents.managerFold')}</summary>
                <p className="text-xs text-muted-foreground">{t('agents.managerHint')}</p>
                <SnippetCard snippet={s} subdued />
              </details>
            ))}
          {h.skill.map((s) => (
            <SnippetCard key={`${h.id}-skill`} snippet={s} />
          ))}
          {h.watch.map((s) => (
            <SnippetCard key={`${h.id}-${s.kind}`} snippet={s} />
          ))}
        </TabsContent>
      ))}
    </Tabs>
  )
}

function Page({ children }: { children: ReactNode }) {
  return <div className="mx-auto w-full max-w-3xl space-y-6 px-5 py-8 sm:px-8 sm:py-10">{children}</div>
}

function CopyPromptButton({ text, label, ok, fail }: { text: string; label: string; ok: string; fail: string }) {
  return (
    <Button type="button" size="sm" variant="outline" onClick={() => void copyWithToast(text, null, ok, fail)}>
      <CopyIcon />
      {label}
    </Button>
  )
}