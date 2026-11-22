import { useTranslation } from '@/i18n/react'
import { SnippetCard } from './SnippetCard'

const removeCommands: Record<string, string | undefined> = {
  'claude-code': 'claude mcp remove --scope user',
  codex: 'codex mcp remove',
}

export function UninstallMcp({ host }: { host: string }) {
  const { t } = useTranslation()
  const command = removeCommands[host]
  if (!command && host !== 'cursor') return null

  return (
    <details className="space-y-2">
      <summary className="cursor-pointer text-ui font-medium">{t('agents.uninstallMcp')}</summary>
      {command ? (
        <div className="space-y-4">
          <SnippetCard snippet={{ kind: 'worker', title: '', lang: 'sh', code: `${command} slate` }} />
          <SnippetCard snippet={{ kind: 'manager', title: '', lang: 'sh', code: `${command} slate-manager` }} />
        </div>
      ) : (
        <p className="text-ui text-muted-foreground">{t('agents.uninstallMcpCursor')}</p>
      )}
    </details>
  )
}
