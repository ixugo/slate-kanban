// 卡片描述：编辑、预览、保存、取消；Cmd/Ctrl+Enter 保存。

import { useState, type KeyboardEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Markdown } from '@/lib/markdown'
import { CARD_DESC_MAX } from '@/lib/limits'

interface DescriptionEditorProps {
  value: string
  onSave: (next: string) => void
  busy?: boolean
}

/** 描述的编辑与预览切换 */
export function DescriptionEditor({ value, onSave, busy }: DescriptionEditorProps) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)

  const startEdit = () => {
    setDraft(value)
    setEditing(true)
  }

  const cancel = () => {
    setDraft(value)
    setEditing(false)
  }

  const save = () => {
    onSave(draft)
    setEditing(false)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault()
      save()
    }
  }

  if (!editing) {
    return (
      <div className="space-y-2">
        <div className="flex gap-2">
          <Button type="button" size="sm" variant="secondary" onClick={startEdit}>
            {t('card.descEdit')}
          </Button>
        </div>
        {value ? <Markdown source={value} /> : <p className="text-ui text-muted-foreground">{t('card.descPlaceholder')}</p>}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <Textarea
        value={draft}
        onChange={(e) => setDraft(e.target.value.slice(0, CARD_DESC_MAX))}
        onKeyDown={onKeyDown}
        rows={8}
        disabled={busy}
        placeholder={t('card.descPlaceholder')}
        aria-label={t('card.field.description')}
      />
      <div className="flex gap-2">
        <Button type="button" size="sm" disabled={busy} onClick={save}>
          {t('common.save')}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={cancel}>
          {t('common.cancel')}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={() => setEditing(false)}>
          {t('card.descPreview')}
        </Button>
      </div>
      <Markdown source={draft} className="max-h-48 overflow-y-auto rounded-xl bg-secondary/50 p-3" />
    </div>
  )
}