// 卡片评论列表与添加；Cmd/Ctrl+Enter 发送。

import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from '@/i18n/react'
import { addComment, deleteComment } from '@/lib/api/endpoints'
import type { Comment } from '@/lib/api/types'
import { Button } from '@/components/ui/button'
import { LazyEditor } from '@/features/editor/LazyEditor'
import { Markdown } from '@/lib/markdown'
import { COMMENT_MAX } from '@/lib/limits'
import { queryKeys } from '@/lib/queryKeys'
import { useErrorToast } from '@/lib/useErrorToast'
import { formatRelative } from '@/lib/format'
import { useLanguage } from '@/i18n/useLanguage'

interface CommentsProps {
  cardId: number
  boardId: number
  comments: Comment[]
}

/** 评论区 */
export function Comments({ cardId, boardId, comments: rawComments }: CommentsProps) {
  const comments = rawComments ?? []
  const { t } = useTranslation()
  const { lang } = useLanguage()
  const [body, setBody] = useState('')
  const queryClient = useQueryClient()
  const toastError = useErrorToast()

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
    },
    onError: toastError,
  })

  /** 作者类型的叫法：人写的叫"我"，系统消息叫"系统"，agent 叫 Agent */
  const kindLabel = (kind: Comment['author_kind']) => t(`card.comment.${kind}` as 'card.comment.human')

  return (
    <section className="space-y-3">
      <h3 className="text-body font-semibold tracking-tight">{t('card.comment.title')}</h3>
      {comments.length === 0 && <p className="text-sm text-muted-foreground">{t('card.comment.empty')}</p>}
      <ul className="space-y-3">
        {comments.map((c) => (
          <li key={c.id} className="rounded-2xl bg-secondary/60 p-3.5">
            <div className="mb-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">
                {c.author_kind === 'agent' ? c.author : kindLabel(c.author_kind)}
              </span>
              {c.author_kind === 'agent' && <span>{kindLabel(c.author_kind)}</span>}
              <span>{formatRelative(c.created_at, lang)}</span>
              <Button
                type="button"
                variant="ghost"
                size="xs"
                className="ml-auto"
                onClick={() => remove.mutate(c.id)}
              >
                {t('common.delete')}
              </Button>
            </div>
            <Markdown source={c.body} />
          </li>
        ))}
      </ul>
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
    </section>
  )
}