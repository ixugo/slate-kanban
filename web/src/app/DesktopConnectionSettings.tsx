import { ExternalLinkIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'

interface DesktopServer {
  port: number
  openPath: string
}

declare global {
  interface Window {
    slateDesktopServer?: DesktopServer
  }
}

export function DesktopConnectionSettings() {
  const { t } = useTranslation()
  const [server, setServer] = useState(() => window.slateDesktopServer)
  const [openState, setOpenState] = useState<'idle' | 'pending' | 'failed' | 'timedOut'>('idle')

  useEffect(() => {
    const sync = () => setServer(window.slateDesktopServer)
    window.addEventListener('slate-desktop-server', sync)
    sync()
    return () => window.removeEventListener('slate-desktop-server', sync)
  }, [])

  if (!server) return null

  async function openBrowser(openPath: string) {
    setOpenState('pending')
    try {
      const res = await fetch(openPath, {
        method: 'POST',
        headers: { 'X-Slate-Desktop': 'browser' },
      })
      if (res.status === 504) {
        setOpenState('timedOut')
      } else {
        setOpenState(res.ok ? 'idle' : 'failed')
      }
    } catch {
      setOpenState('failed')
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <dl className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <dt className="font-medium">{t('preferences.listeningPort')}</dt>
          <dd className="font-mono tabular-nums">{server.port}</dd>
        </dl>
        {server.openPath && (
          <Button type="button" variant="outline" disabled={openState === 'pending'} onClick={() => void openBrowser(server.openPath)}>
            <ExternalLinkIcon aria-hidden data-icon="inline-start" />
            {t('preferences.openInBrowser')}
          </Button>
        )}
      </div>
      {(openState === 'failed' || openState === 'timedOut') && (
        <p role="alert" className="text-destructive">
          {t(openState === 'timedOut' ? 'preferences.browserOpenTimedOut' : 'preferences.browserOpenFailed')}
        </p>
      )}
    </div>
  )
}
