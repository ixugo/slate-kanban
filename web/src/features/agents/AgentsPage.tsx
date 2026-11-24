// Agent 接入页：按提示词、手动接入和桌面派发展示各自的设置。

import { CopyIcon } from 'lucide-react'
import { useState, useSyncExternalStore, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useBoards } from '@/features/boards/useBoards'
import type { Board, ConnectGuide } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { useDocumentTitle } from '@/lib/useDocumentTitle'
import { copyWithToast } from './copyText'
import { SnippetCard } from './SnippetCard'
import { useConnectGuide } from './useConnectGuide'
import { parseAgentsProjectParam } from '@/lib/agentsProject'
import { bindWatchPrompt } from './watchPrompt'
import { WatchSection } from './WatchSection'
import { UninstallMcp } from './UninstallMcp'
import { DesktopDispatch } from './DesktopDispatch'

function subscribeDesktopDispatch(onChange: () => void) {
  window.addEventListener('slate-desktop-dispatch', onChange)
  return () => window.removeEventListener('slate-desktop-dispatch', onChange)
}

/** 接入页默认导出 */
export default function AgentsPage() {
  const { t } = useTranslation()
  const { data, error, isPending, isError, refetch } = useConnectGuide()
  const [params] = useSearchParams()
  const project = parseAgentsProjectParam(params.get('project'))
  const [selectedId, setSelectedId] = useState('')
  const [method, setMethod] = useState('prompt')
  const desktopBridge = useSyncExternalStore(subscribeDesktopDispatch, () => window.slateDesktopDispatch)
  const { data: boards, error: boardsError, isPending: boardsPending } = useBoards()
  // URL 里的 project 能对应上看板就选中它并锁定下拉框；对应不上时不锁，仍可手动选。
  const paramBoard = project === null ? undefined : boards?.find((b) => b.slug === project)
  const board = paramBoard ?? boards?.find((b) => String(b.id) === selectedId)
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
  const activeMethod = method === 'desktop' && !desktopBridge ? 'prompt' : method
  const methods = [
    { id: 'prompt', label: t('agents.methods.prompt') },
    { id: 'manual', label: t('agents.methods.manual') },
  ]
  if (desktopBridge) methods.push({ id: 'desktop', label: t('agents.methods.desktop') })
  const watch = (
    <WatchSection
      guide={data}
      board={board}
      boards={boards}
      error={boardsError}
      isPending={boardsPending}
      locked={paramBoard !== undefined}
      invalidProject={project !== null && boards !== undefined && paramBoard === undefined}
      onSelect={setSelectedId}
    />
  )

  return (
    <Page>
      <header className="space-y-1.5">
        <h1 className="text-display font-semibold tracking-tight">{t('agents.title')}</h1>
        <p className="text-body text-muted-foreground">{t('agents.subtitle')}</p>
      </header>
      <Tabs value={activeMethod} onValueChange={setMethod} className="gap-4">
        <TabsList
          aria-label={t('agents.methods.title')}
          variant="line"
          className="w-full items-stretch border-b p-0 group-data-horizontal/tabs:h-auto"
        >
          {methods.map(({ id, label }, index) => {
            // WKWebView 可能保留共享 Tabs 上一项的伪元素样式。
            const selected = activeMethod === id
            return (
              <TabsTrigger
                key={id}
                value={id}
                className="h-auto min-h-12 min-w-0 basis-0 flex-col gap-0.5 rounded-none px-2 py-2 text-center whitespace-normal after:hidden data-active:shadow-none sm:flex-row sm:gap-2"
              >
                <span className="shrink-0 text-xs text-muted-foreground">
                  {t('agents.methods.number', { number: index + 1 })}
                </span>{' '}
                <span className={selected ? 'text-foreground' : 'text-muted-foreground'}>{label}</span>
                {selected && (
                  <span aria-hidden="true" className="pointer-events-none absolute inset-x-0 -bottom-px h-0.5 bg-foreground" />
                )}
              </TabsTrigger>
            )
          })}
        </TabsList>
        <TabsContent value="prompt" className="space-y-4">
          <section className="space-y-2.5 rounded-2xl bg-card p-5 shadow-card">
            <h2 className="text-body font-semibold tracking-tight">{t('agents.promptTitle')}</h2>
            <p className="text-ui wrap-anywhere">{prompt}</p>
            <CopyPromptButton text={prompt} label={t('common.copy')} ok={t('common.copied')} fail={t('agents.copyFailed')} />
          </section>
          {watch}
        </TabsContent>
        <TabsContent value="manual" className="space-y-4">
          <section className="space-y-4 rounded-2xl bg-card p-5 shadow-card">
            <h2 className="text-body font-semibold tracking-tight">{t('agents.advanced')}</h2>
            <p className="text-ui text-muted-foreground">{t('agents.advancedHint')}</p>
            <HostTabs guide={data} board={board} />
            <div className="space-y-2 text-ui text-muted-foreground">
              <p>{t('agents.skillOptional')}</p>
              <p>{t('agents.nameHint')}</p>
              <p>{t('agents.managerHint')}</p>
            </div>
          </section>
          {watch}
        </TabsContent>
        {desktopBridge && (
          <TabsContent value="desktop" forceMount hidden={activeMethod !== 'desktop'}>
            <DesktopDispatch bridge={desktopBridge} boards={boards} />
          </TabsContent>
        )}
      </Tabs>
    </Page>
  )
}

/** 各宿主的 MCP、skill 和守候说明 */
function HostTabs({ guide: data, board }: { guide: ConnectGuide; board?: Board }) {
  return (
    <Tabs defaultValue={data.hosts[0]?.id ?? 'default'}>
      <TabsList className="max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto">
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
          <UninstallMcp host={h.id} />
          {h.skill.map((s) => (
            <SnippetCard key={`${h.id}-skill`} snippet={s} />
          ))}
          {h.watch.map((s) => (
            <SnippetCard
              key={`${h.id}-${s.kind}`}
              snippet={board ? { ...s, code: bindWatchPrompt(s.code, board, h.id === 'codex') } : s}
            />
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
