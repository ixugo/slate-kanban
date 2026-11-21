// 怎么持续监听：讲清长连接推送的原理，给一行守候提示词，按 URL 的 project 参数（锁定）或用户选择确定看板。

import { useTranslation } from '@/i18n/react'
import type { Board, ConnectGuide } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { SnippetCard } from './SnippetCard'
import { bindWatchPrompt } from './watchPrompt'

interface WatchSectionProps {
  guide: ConnectGuide
  board: Board | undefined
  boards: Board[] | undefined
  error: Error | null
  isPending: boolean
  /** URL 的 project 对上了看板：下拉框选中该看板并禁用。 */
  locked: boolean
  /** URL 带了 project 但对应不上任何看板：给出提示，下拉框仍可手动选。 */
  invalidProject: boolean
  onSelect: (id: string) => void
}

/** 持续监听一节：确定看板后才生成守候提示词。 */
export function WatchSection({ guide, board, boards, error, isPending, locked, invalidProject, onSelect }: WatchSectionProps) {
  const { t } = useTranslation()

  return (
    <section className="space-y-4 rounded-2xl bg-card p-5 shadow-card">
      <div className="space-y-1.5">
        <h2 className="text-body font-semibold tracking-tight">{t('agents.watch.title')}</h2>
        <p className="text-ui text-muted-foreground">{t('agents.watch.intro')}</p>
      </div>
      <ol className="list-decimal space-y-1.5 pl-5 text-ui marker:font-semibold">
        <li>{t('agents.watch.how.connect')}</li>
        <li>{t('agents.watch.how.push')}</li>
        <li>{t('agents.watch.how.loop')}</li>
      </ol>
      {isPending && (
        <p role="status" className="text-ui text-muted-foreground">{t('common.loading')}</p>
      )}
      {!boards && error && (
        <p role="alert" className="text-ui text-destructive">{describeError(error, t).message}</p>
      )}
      {boards && (
        <label className="flex items-center gap-2 text-ui">
          {t('agents.watch.board')}
          <select
            value={board?.id ?? ''}
            disabled={locked || boards.length === 0}
            onChange={(e) => onSelect(e.target.value)}
            className="h-9 min-w-0 rounded-xl border border-input bg-background px-3 text-ui"
          >
            <option value="" disabled>
              {boards.length > 0 ? t('agents.watch.selectBoard') : t('boards.empty.title')}
            </option>
            {boards.map((b) => (
              <option key={b.id} value={String(b.id)}>
                {b.slug} {b.name}
              </option>
            ))}
          </select>
        </label>
      )}
      {invalidProject && (
        <p role="alert" className="text-ui text-destructive">{t('agents.watch.invalidProject')}</p>
      )}
      {board && (
        <SnippetCard snippet={{
          kind: 'prompt',
          title: t('agents.watch.loopTitle'),
          lang: 'sh',
          code: bindWatchPrompt(guide.loop_prompt, board),
        }} />
      )}
      <ul className="list-disc space-y-1.5 pl-5 text-ui text-muted-foreground">
        <li>{t('agents.watch.notes.resume')}</li>
        <li>{t('agents.watch.notes.perm')}</li>
      </ul>
    </section>
  )
}
