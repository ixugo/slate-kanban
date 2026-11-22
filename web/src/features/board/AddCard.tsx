// 列底"添加任务"：Enter 提交、Shift+Enter 换行、Esc 取消。

import { useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { CARD_TITLE_MAX } from '@/lib/limits'
import { useOnlineStatus } from '@/pwa/useOnlineStatus'
import { PlusIcon } from 'lucide-react'

interface AddCardProps {
  open: boolean
  onOpen: () => void
  onClose: () => void
  onSubmit: (title: string) => void
  busy?: boolean
}

/** 列底部的新建任务控件；离线时整个看板只读，不提供新建入口 */
export function AddCard({ open, onOpen, onClose, onSubmit, busy }: AddCardProps) {
  const { t } = useTranslation()
  const online = useOnlineStatus()
  const [title, setTitle] = useState('')
  const ref = useRef<HTMLTextAreaElement>(null)

  const submit = () => {
    const trimmed = title.trim()
    if (!trimmed) return
    onSubmit(trimmed)
    setTitle('')
  }

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Escape') {
      e.preventDefault()
      setTitle('')
      onClose()
    } else if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      submit()
    }
  }

  if (!online) return null

  if (!open) {
    return (
      <Button type="button" variant="ghost" size="sm" className="w-full justify-start text-muted-foreground hover:bg-foreground/[0.06]" onClick={onOpen}>
        <PlusIcon />
        {t('board.addCard')}
      </Button>
    )
  }

  return (
    <div className="space-y-2">
      <Textarea
        ref={ref}
        value={title}
        rows={2}
        maxLength={CARD_TITLE_MAX}
        placeholder={t('board.addCardPlaceholder')}
        onChange={(e) => setTitle(e.target.value)}
        onKeyDown={onKeyDown}
        // 点开就能直接打字；提交中用只读而不是 disabled，禁用会让输入框失去焦点，连续添加还得再点一次
        autoFocus
        readOnly={busy}
        aria-label={t('board.addCard')}
      />
      <div className="flex gap-2">
        <Button type="button" size="sm" disabled={busy || !title.trim()} onClick={submit}>
          {t('common.create')}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onClose}>
          {t('common.cancel')}
        </Button>
      </div>
    </div>
  )
}