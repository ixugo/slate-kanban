// 顶栏的「关于」按钮：点开才加载弹窗，不增加首屏体积。

import { InfoIcon } from 'lucide-react'
import { lazy, Suspense, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'

/** 弹窗体积不大但首屏用不到，第一次点开时再加载 */
const AboutDialog = lazy(() => import('./AboutDialog').then((m) => ({ default: m.AboutDialog })))

/** 顶栏图标按钮，打开「关于」弹窗 */
export function AboutButton() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [loaded, setLoaded] = useState(false)
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={t('nav.about')}
        title={t('nav.about')}
        onClick={() => {
          setLoaded(true)
          setOpen(true)
        }}
      >
        <InfoIcon />
      </Button>
      {loaded && (
        <Suspense fallback={null}>
          <AboutDialog open={open} onOpenChange={setOpen} />
        </Suspense>
      )}
    </>
  )
}
