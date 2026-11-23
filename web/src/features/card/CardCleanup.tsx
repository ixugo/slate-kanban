import { useEffect, useState, useSyncExternalStore } from 'react'
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'
import type { CardDetail } from '@/lib/api/types'

type CleanupAction = 'cleanup-status' | 'cleanup-preview' | 'cleanup-retry' | 'cleanup-archive'
interface CleanupStatus {
  status: 'unknown' | 'pending' | 'blocked' | 'cleaned'
  reason?: string
  destination?: string
  files?: string[]
  preview_token?: string
}

declare global {
  interface Window {
    slateLocalCleanup?: string
  }
}

function subscribe(callback: () => void) {
  window.addEventListener('slate-local-cleanup', callback)
  return () => window.removeEventListener('slate-local-cleanup', callback)
}

function getEndpoint() {
  return window.slateLocalCleanup
}

async function requestCleanup(endpoint: string, cardID: number, action: CleanupAction, token?: string, signal?: AbortSignal): Promise<CleanupStatus> {
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ card_id: cardID, action, preview_token: token }),
    signal,
  })
  if (!response.ok) throw new Error((await response.text()).trim())
  return response.json()
}

export function CardCleanup({ card }: { card: Pick<CardDetail, 'id' | 'done' | 'merged_commits'> }) {
  const endpoint = useSyncExternalStore(subscribe, getEndpoint)
  const commits = card.merged_commits ?? []
  if (!card.done || commits.length === 0) return null
  return (
    <section className="grid gap-2 rounded-lg border border-border p-3 text-sm" aria-label={commits.join(', ')}>
      <MergeCommits commits={commits} />
      {endpoint && <LocalCleanup key={card.id} endpoint={endpoint} cardID={card.id} />}
    </section>
  )
}

function MergeCommits({ commits }: { commits: string[] }) {
  const { t } = useTranslation()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="font-medium">{t('card.cleanup.commits')}</span>
      {commits.map((commit) => <code key={commit} className="rounded-sm bg-muted px-1.5 font-mono">{commit}</code>)}
    </div>
  )
}

function LocalCleanup({ endpoint, cardID }: { endpoint: string; cardID: number }) {
  const { t } = useTranslation()
  const [status, setStatus] = useState<CleanupStatus>()
  const [preview, setPreview] = useState<CleanupStatus>()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    void requestCleanup(endpoint, cardID, 'cleanup-status', undefined, controller.signal).then((value) => {
      if (!controller.signal.aborted) setStatus(value)
    }).catch((cause: Error) => {
      if (!controller.signal.aborted) setError(cause.message)
    })
    return () => controller.abort()
  }, [cardID, endpoint])

  async function inspect(action: 'cleanup-preview' | 'cleanup-retry') {
    setBusy(true)
    setError('')
    try {
      const value = await requestCleanup(endpoint, cardID, action)
      if (value.preview_token) setPreview(value)
      else setStatus(value)
    } catch (cause) {
      setError(String((cause as Error).message))
    } finally {
      setBusy(false)
    }
  }

  async function confirmArchive() {
    if (!preview?.preview_token) return
    setBusy(true)
    setError('')
    try {
      setStatus(await requestCleanup(endpoint, cardID, 'cleanup-archive', preview.preview_token))
    } catch (cause) {
      setError(String((cause as Error).message))
    } finally {
      setBusy(false)
      setPreview(undefined)
    }
  }

  return (
    <div className="grid gap-2" aria-live="polite">
      <div>{t(`card.cleanup.${status?.status ?? (error ? 'blocked' : 'loading')}`)}</div>
      {status?.reason && <p className="wrap-anywhere text-muted-foreground">{status.reason}</p>}
      {status?.status === 'cleaned' && status.destination && <code className="wrap-anywhere text-xs">{status.destination}</code>}
      {error && <p role="alert" className="wrap-anywhere text-destructive">{error}</p>}
      {status && (status.status === 'blocked' || status.status === 'pending') && (
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="secondary" size="sm" onClick={() => void inspect('cleanup-preview')} disabled={busy}>
            {t('card.cleanup.preview')}
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => void inspect('cleanup-retry')} disabled={busy}>
            {t('card.cleanup.retry')}
          </Button>
        </div>
      )}
      <AlertDialog open={!!preview} onOpenChange={(open) => { if (!open && !busy) setPreview(undefined) }}>
        <AlertDialogContent className="sm:max-w-xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t('card.cleanup.confirm')}</AlertDialogTitle>
            <AlertDialogDescription>{t('card.cleanup.summary')}</AlertDialogDescription>
          </AlertDialogHeader>
          <div className="grid min-w-0 gap-2 text-sm">
            <span className="font-medium">{t('card.cleanup.destination')}</span>
            <code className="wrap-anywhere text-xs">{preview?.destination}</code>
            <ul className="max-h-44 overflow-y-auto rounded-md bg-muted px-3 py-2 font-mono text-xs">
              {preview?.files?.map((path) => <li key={path} className="wrap-anywhere whitespace-pre-wrap">{path}</li>)}
            </ul>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>{t('card.cleanup.keep')}</AlertDialogCancel>
            <Button type="button" variant="destructive" disabled={busy} onClick={() => void confirmArchive()}>{t('card.cleanup.archive')}</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
