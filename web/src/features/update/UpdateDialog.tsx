// 发现新版本的弹窗：显示版本号和更新说明，两个按钮：此版本忽略、立即更新。

import { useMutation } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useTranslation } from '@/i18n/react'
import { applyUpdate } from '@/lib/api/endpoints'
import type { UpdateInfo } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { Markdown } from '@/lib/markdown'
import { useReloadAfterRestart } from './useReloadAfterRestart'
import { saveIgnored } from './version'

interface Props {
  info: UpdateInfo
  onClose: () => void
  onCloseAutoFocus?: (event: Event) => void
}

/** 只放行 https 链接，免得服务端返回的地址被当成脚本 */
function safeUrl(url: string): string {
  return url.startsWith('https://') ? url : ''
}

/** 更新弹窗；更新成功后服务端会自动重启，页面等新版本起来后自动刷新；不能自动重启时提示用户手动重启 */
export default function UpdateDialog({ info, onClose, onCloseAutoFocus }: Props) {
  const { t } = useTranslation()
  const update = useMutation({ mutationFn: applyUpdate })
  const restarting = update.data?.restarting === true
  const restartTimedOut = useReloadAfterRestart(restarting, update.data?.version ?? '')
  const resultMessage = update.data?.installer_opened
    ? 'update.installerOpened'
    : restarting && !restartTimedOut ? 'update.restarting' : 'update.done'
  const link = safeUrl(info.url)
  const ignore = () => {
    saveIgnored(info.latest)
    onClose()
  }
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-lg" onCloseAutoFocus={onCloseAutoFocus}>
        <DialogHeader>
          <DialogTitle>{t('update.title')}</DialogTitle>
          <DialogDescription>{t('update.versions', { current: info.current, latest: info.latest })}</DialogDescription>
        </DialogHeader>
        {info.notes !== '' && <Markdown source={info.notes} className="max-h-72 overflow-auto text-ui" />}
        {update.isSuccess && (
          <p role="status">
            {t(resultMessage, { version: update.data.version })}
          </p>
        )}
        {update.isError && <p role="alert" className="text-destructive">{describeError(update.error, t).message}</p>}
        {!update.isSuccess && (
          <DialogFooter>
            <Button type="button" variant="outline" onClick={ignore}>
              {t('update.ignore')}
            </Button>
            {info.can_apply || link === '' ? (
              <Button type="button" disabled={update.isPending || !info.can_apply} onClick={() => update.mutate()}>
                {update.isPending ? t('update.updating') : t('update.now')}
              </Button>
            ) : (
              <Button asChild>
                <a href={link} target="_blank" rel="noopener noreferrer">
                  {t('update.download')}
                </a>
              </Button>
            )}
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
