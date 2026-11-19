// 长期挂着：给两段提示词（长期守候、单轮），可以选看板把看板名填进去。

import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { useBoards } from '@/features/boards/useBoards'
import type { ConnectGuide } from '@/lib/api/types'
import { SnippetCard } from './SnippetCard'

/** 提示词里的看板名占位符，和服务端 connectapi 的 boardPlaceholder 一致 */
const BOARD_PLACEHOLDER = '<看板名>'

/** 把占位符换成所选看板的名字；没选就原样保留，让用户自己改 */
function fillBoard(prompt: string, name: string): string {
  return name ? prompt.replaceAll(BOARD_PLACEHOLDER, name) : prompt
}

/** 长期挂着一节：看板选择、两段提示词和注意事项；看板列表读不到时选择框不出现，占位符保留 */
export function WatchSection({ guide }: { guide: ConnectGuide }) {
  const { t } = useTranslation()
  const [board, setBoard] = useState('')
  const boards = useBoards().data ?? []
  const prompt = (code: string, title: string) => ({ kind: 'prompt', title, lang: 'sh' as const, code: fillBoard(code, board) })

  return (
    <section className="space-y-4 rounded-2xl bg-card p-5 shadow-card">
      <div className="space-y-1.5">
        <h2 className="text-body font-semibold tracking-tight">{t('agents.watch.title')}</h2>
        <p className="text-ui text-muted-foreground">{t('agents.watch.intro')}</p>
      </div>
      {boards.length > 0 && (
        <label className="flex items-center gap-2 text-ui">
          {t('agents.watch.board')}
          <select
            value={board}
            onChange={(e) => setBoard(e.target.value)}
            className="h-9 rounded-xl border border-input bg-background px-3 text-ui"
          >
            <option value="">{t('agents.watch.anyBoard')}</option>
            {boards.map((b) => (
              <option key={b.id} value={b.name}>
                {b.name}
              </option>
            ))}
          </select>
        </label>
      )}
      <SnippetCard snippet={prompt(guide.watch_prompt, t('agents.watch.longTitle'))} />
      <SnippetCard snippet={prompt(guide.loop_prompt, t('agents.watch.loopTitle'))} />
      <ul className="list-disc space-y-1.5 pl-5 text-ui text-muted-foreground">
        <li>{t('agents.watch.notes.how')}</li>
        <li>{t('agents.watch.notes.resume')}</li>
        <li>{t('agents.watch.notes.perm')}</li>
      </ul>
    </section>
  )
}
