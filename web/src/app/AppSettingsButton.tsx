// 设置只在打开后加载，保持看板首屏轻量。

import { SettingsIcon } from 'lucide-react'
import { lazy, Suspense, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'

const AppSettingsDialog = lazy(() => import('./AppSettingsDialog'))

export function AppSettingsButton() {
  const { t } = useTranslation()
  const trigger = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button
        ref={trigger}
        type="button"
        variant="ghost"
        size="icon"
        aria-label={t('nav.settings')}
        aria-haspopup="dialog"
        aria-expanded={open}
        title={t('nav.settings')}
        onClick={() => setOpen(true)}
      >
        <SettingsIcon />
      </Button>
      {open && (
        <Suspense fallback={null}>
          <AppSettingsDialog
            onClose={() => setOpen(false)}
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
