// 卡片的「评论和活动」：最上面是写评论的编辑器，下面把评论和这张卡片发生过的事（建卡、换列、认领、自动化……）
// 按时间从新到旧排在一起；「显示详情」可以收起事件，只看评论。Ctrl/Cmd+Enter 发送评论。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { BotIcon, UserIcon, ZapIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useLanguage } from '@/i18n/useLanguage'
import { addComment, deleteComment } from '@/lib/api/endpoints'
import type { ActorKind, BoardEvent, Comment } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { LazyEditor } from '@/features/editor/LazyEditor'
import { actorLabel, describeEvent } from '@/features/activity/describeEvent'
import { Markdown } from '@/lib/markdown'
import { COMMENT_MAX } from '@/lib/limits'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { formatDateTime, formatRelative } from '@/lib/format'
import { useCardActivity } from './useCardActivity'
import { buildTimeline } from './timeline'

interface CardTimelineProps {
  cardId: number
  boardId: number
  comments: Comment[]
}

const ACTOR_ICONS: Record<ActorKind, typeof UserIcon> = { human: UserIcon, agent: BotIcon, system: ZapIcon }

/** 时间线上的一条事件：图标、谁做了什么、补充说明和时间 */
function EventLine({ event }: { event: BoardEvent }) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const { text, detail } = describeEvent(event, t)
  const Icon = ACTOR_ICONS[event.actor_kind] ?? ZapIcon
  return (
    <li className="flex gap-2.5 px-1 text-ui">
      <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-secondary text-muted-foreground">
        <Icon className="size-3" aria-hidden />
      </span>
      <div className="min-w-0 flex-1">
        <p className="break-words">
          <span className="font-medium">{actorLabel({ kind: event.actor_kind, name: event.actor }, t)}</span>{' '}
          <span className="text-muted-foreground">{text}</span>
        </p>
        {detail !== '' && <p className="line-clamp-2 text-xs break-words text-muted-foreground">{detail}</p>}
        <time dateTime={event.created_at} title={formatDateTime(event.created_at, lang)} className="text-[0.6875rem] text-muted-foreground">
          {formatRelative(event.created_at, lang)}
        </time>
      </div>
    </li>
  )
}

/** 评论和活动 */
export function CardTimeline({ cardId, boardId, comments }: CardTimelineProps) {
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [body, setBody] = useState('')
  const [showDetails, setShowDetails] = useState(true)
  const queryClient = useQueryClient()
  const toastError = useErrorToast()
  const activity = useCardActivity(boardId, cardId, showDetails)
  const items = useMemo(() => buildTimeline(comments ?? [], activity.data?.events ?? [], showDetails), [comments, activity.data, showDetails])

  const send = useMutation({
    mutationFn: () => addComment(cardId, body.trim().slice(0, COMMENT_MAX)),
    onSuccess: () => {
      setBody('')
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(cardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })

  const remove = useMutation({
    mutationFn: (id: number) => deleteComment(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.card(cardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
    },
    onError: toastError,
  })

  /** 作者类型的叫法：人写的叫"我"，系统消息叫"系统"，agent 叫 Agent */
  const kindLabel = (kind: Comment['author_kind']) => t(`card.comment.${kind}` as 'card.comment.human')

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-body font-semibold tracking-tight">{t('card.timeline.title')}</h3>
        <Button type="button" variant="ghost" size="xs" aria-pressed={showDetails} onClick={() => setShowDetails((v) => !v)}>
          {showDetails ? t('card.timeline.hideDetails') : t('card.timeline.showDetails')}
        </Button>
      </div>
      <LazyEditor
        compact
        value={body}
        onChange={setBody}
        max={COMMENT_MAX}
        placeholder={t('card.comment.placeholder')}
        label={t('card.comment.title')}
        onSubmit={() => body.trim() && send.mutate()}
      />
      <Button type="button" size="sm" disabled={!body.trim() || send.isPending} onClick={() => send.mutate()}>
        {t('card.comment.send')}
      </Button>
      {items.length === 0 && <p className="text-sm text-muted-foreground">{t('card.comment.empty')}</p>}
      <ul className="space-y-3">
        {items.map((item) =>
          item.kind === 'event' ? (
            <EventLine key={`e${item.id}`} event={item.event} />
          ) : (
            <li key={`c${item.id}`} className="rounded-2xl bg-secondary/60 p-3.5">
              <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className="font-medium text-foreground">
                  {item.comment.author_kind === 'agent' ? item.comment.author : kindLabel(item.comment.author_kind)}
                </span>
                {item.comment.author_kind === 'agent' && <span>{kindLabel(item.comment.author_kind)}</span>}
                <span>{formatRelative(item.comment.created_at, lang)}</span>
                <Button type="button" variant="ghost" size="xs" className="ml-auto" onClick={() => remove.mutate(item.comment.id)}>
                  {t('common.delete')}
                </Button>
              </div>
              <Markdown source={item.comment.body} />
            </li>
          ),
        )}
      </ul>
    </section>
  )
}
