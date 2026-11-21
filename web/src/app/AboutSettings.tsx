// 设置里的「关于」：项目介绍、服务端版本及项目仓库。

import { useQuery } from '@tanstack/react-query'
import { useTranslation } from '@/i18n/react'
import { getVersion } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { Logo } from './Logo'

/** 项目的公开仓库地址 */
export const PROJECT_URL = 'https://github.com/ixugo/slate-kanban'

/** GitHub 的标志图形（Octicons 的 mark-github），用当前文字颜色填充，装饰用 */
function GithubMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden="true" fill="currentColor" className={className}>
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
    </svg>
  )
}

export function AboutSettings() {
  const { t } = useTranslation()
  const version = useQuery({
    queryKey: queryKeys.version,
    queryFn: ({ signal }) => getVersion(signal),
    staleTime: Infinity,
  })
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <Logo className="size-9 shrink-0" />
        <h2 className="text-body font-semibold tracking-tight">{t('about.title')}</h2>
      </div>
      <p className="text-muted-foreground">{t('about.intro')}</p>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <dl className="flex min-w-0 items-baseline gap-2">
          <dt className="text-muted-foreground">{t('about.version')}</dt>
          <dd className="font-mono break-all">{version.data?.version ?? (version.isError ? t('about.versionUnknown') : '…')}</dd>
        </dl>
        <a
          href={PROJECT_URL}
          target="_blank"
          rel="noopener noreferrer"
          aria-label="GitHub"
          title="GitHub"
          className="inline-flex size-8 items-center justify-center rounded-full text-foreground/80 outline-none transition-colors hover:bg-secondary hover:text-foreground focus-visible:ring-[3px] focus-visible:ring-ring/40"
        >
          <GithubMark className="size-4" />
        </a>
      </div>
    </div>
  )
}
