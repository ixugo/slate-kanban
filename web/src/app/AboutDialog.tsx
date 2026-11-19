// 「关于」弹窗：一句话介绍、当前服务端的版本号和项目地址。版本号在弹窗打开时才向服务端取。

import { useQuery } from '@tanstack/react-query'
import { ExternalLinkIcon } from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useTranslation } from '@/i18n/react'
import { getVersion } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { Logo } from './Logo'

/** 项目的公开仓库地址 */
export const PROJECT_URL = 'https://github.com/ixugo/slate-kanban'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/** 关于弹窗；取版本失败时显示「未知」，不影响其余内容 */
export function AboutDialog({ open, onOpenChange }: Props) {
  const { t } = useTranslation()
  const version = useQuery({
    queryKey: queryKeys.version,
    queryFn: ({ signal }) => getVersion(signal),
    enabled: open,
    staleTime: Infinity,
  })
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader className="items-center text-center sm:text-center">
          <Logo className="size-12" />
          <DialogTitle>{t('about.title')}</DialogTitle>
          <DialogDescription>{t('about.intro')}</DialogDescription>
        </DialogHeader>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-ui">
          <dt className="text-muted-foreground">{t('about.version')}</dt>
          <dd className="font-mono">{version.data?.version ?? (version.isError ? t('about.versionUnknown') : '…')}</dd>
          <dt className="text-muted-foreground">{t('about.project')}</dt>
          <dd className="min-w-0">
            <a
              href={PROJECT_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex max-w-full items-center gap-1 break-all underline underline-offset-4 outline-none hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/40"
            >
              {PROJECT_URL}
              <ExternalLinkIcon className="size-3.5 shrink-0" aria-hidden />
            </a>
          </dd>
        </dl>
      </DialogContent>
    </Dialog>
  )
}
