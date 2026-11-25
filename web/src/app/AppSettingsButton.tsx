// 设置只在打开后加载，保持看板首屏轻量。

import { SettingsIcon } from 'lucide-react'
import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'

const AppSettingsDialog = lazy(() => import('./AppSettingsDialog'))

export function AppSettingsButton() {
  const { t } = useTranslation()
  const trigger = useRef<HTMLButtonElement>(null)
  const [tab, setTab] = useState<string | null>(null)

  useEffect(() => {
    const html = document.documentElement
    const attribute = 'data-slate-settings-tab'
    const openRequestedTab = () => {
      const requestedTab = html.getAttribute(attribute)
      if (requestedTab !== 'general' && requestedTab !== 'about') return
      html.removeAttribute(attribute)
      setTab(requestedTab)
    }
    const observer = new MutationObserver(openRequestedTab)
    observer.observe(html, { attributeFilter: [attribute] })
    openRequestedTab()
    return () => observer.disconnect()
  }, [])

  return (
    <>
      <Button
        ref={trigger}
        type="button"
        variant="ghost"
        size="icon"
        aria-label={t('nav.settings')}
        aria-haspopup="dialog"
        aria-expanded={tab !== null}
        title={t('nav.settings')}
        onClick={() => setTab('general')}
      >
        <SettingsIcon />
      </Button>
      {tab !== null && (
        <Suspense fallback={null}>
          <AppSettingsDialog
            tab={tab}
            onTabChange={setTab}
            onClose={() => setTab(null)}
            onCloseAutoFocus={(event) => {
              event.preventDefault()
              trigger.current?.focus()
            }}
          />
        </Suspense>
      )}
    </>
  )
}
