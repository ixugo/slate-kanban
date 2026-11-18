// 一段可复制的安装命令或配置。

import { CopyIcon } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import type { Snippet } from '@/lib/api/types'
import { copyWithToast } from './copyText'

const KIND_TITLE: Record<string, string> = {
  worker: 'agents.snippets.worker',
  manager: 'agents.snippets.manager',
  skill: 'agents.snippets.skill',
}

interface Props {
  snippet: Snippet
  subdued?: boolean
}

/** 代码块 + 复制按钮 */
export function SnippetCard({ snippet, subdued }: Props) {
  const { t } = useTranslation()
  const preRef = useRef<HTMLPreElement>(null)
  const titleKey = KIND_TITLE[snippet.kind] as 'agents.snippets.worker' | 'agents.snippets.manager' | 'agents.snippets.skill' | undefined
  const heading = titleKey ? t(titleKey) : snippet.title

  const onCopy = () => {
    void copyWithToast(snippet.code, preRef.current, t('common.copied'), t('agents.copyFailed'))
  }

  return (
    <article className={subdued ? 'opacity-90' : undefined}>
      <SnippetHeader title={heading} onCopy={onCopy} t={t} />
      <pre
        ref={preRef}
        className="mt-2 max-h-60 overflow-auto border border-border bg-muted/50 p-3 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap"
      >
        {snippet.code}
      </pre>
    </article>
  )
}

function SnippetHeader({
  title,
  onCopy,
  t,
}: {
  title: string
  onCopy: () => void
  t: (k: 'common.copy') => string
}) {
  return (
    <div className="flex items-center justify-between gap-2">
      <h3 className="text-sm font-medium">{title}</h3>
      <Button type="button" size="sm" variant="outline" onClick={onCopy}>
        <CopyIcon />
        {t('common.copy')}
      </Button>
    </div>
  )
}