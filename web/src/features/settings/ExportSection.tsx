// 设置页「导出」：把整个项目下载成 JSON（完整数据）或 Markdown（可读文档）。

import { DownloadIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { API_PREFIX } from '@/lib/api/client'
import type { Board } from '@/lib/api/types'

interface Props {
  board: Board
}

/** 能导出的格式，取值就是导出接口的 format 参数；各格式的文案放在 settings.export 下 */
const FORMATS = ['json', 'md'] as const

/**
 * 导出页签：每种格式一行，写清用途并给一个下载按钮。
 * 下载直接用带 download 的链接交给浏览器处理：文件名由服务端的 Content-Disposition 决定，
 * 不需要把整个项目读进前端内存，也不占用接口缓存。
 */
export function ExportSection({ board }: Props) {
  const { t } = useTranslation()
  return (
    <div className="max-w-xl space-y-2 rounded-2xl bg-card p-5 shadow-card">
      <p className="text-ui text-muted-foreground">{t('settings.export.body')}</p>
      <ul className="divide-y divide-border">
        {FORMATS.map((format) => (
          <li key={format} className="flex items-center justify-between gap-4 py-3">
            <div className="min-w-0">
              <p className="text-ui font-medium">{t(`settings.export.${format}`)}</p>
              <p className="text-xs text-muted-foreground">{t(`settings.export.${format}Hint`)}</p>
            </div>
            <Button variant="outline" size="sm" asChild>
              <a href={`${API_PREFIX}/boards/${board.id}/export?format=${format}`} download>
                <DownloadIcon />
                {t('settings.export.download')}
              </a>
            </Button>
          </li>
        ))}
      </ul>
    </div>
  )
}
