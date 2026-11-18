// 卡片标签编辑：回车或逗号添加，× 删除，空输入退格删最后一枚。

import { useState, type KeyboardEvent } from 'react'
import { useTranslation } from '@/i18n/react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { XIcon } from 'lucide-react'
import { CARD_TAG_MAX, CARD_TAGS_MAX } from '@/lib/limits'
import { applyTag, popTag, removeTag } from './tagList'

interface TagEditorProps {
  tags: string[]
  onChange: (tags: string[]) => void
}

/** 标签输入与列表 */
export function TagEditor({ tags: rawTags, onChange }: TagEditorProps) {
  const tags = rawTags ?? []
  const { t } = useTranslation()
  const [draft, setDraft] = useState('')
  const [hint, setHint] = useState('')

  const commit = (raw: string) => {
    const result = applyTag(tags, raw)
    if (!result.ok) {
      if (result.reason === 'limit') setHint(t('card.tagLimit', { max: String(CARD_TAGS_MAX) }))
      else if (result.reason === 'tooLong') setHint(t('card.tagTooLong', { max: String(CARD_TAG_MAX) }))
      else setHint('')
      return
    }
    setHint('')
    onChange(result.tags)
    setDraft('')
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault()
      commit(draft)
      return
    }
    if (e.key === 'Backspace' && draft === '' && tags.length > 0) {
      e.preventDefault()
      onChange(popTag(tags))
    }
  }

  return (
    <div className="space-y-1.5">
      <Label htmlFor="card-tags">{t('card.field.tags')}</Label>
      <div className="flex flex-wrap gap-1">
        {tags.map((tag) => (
          <Badge key={tag} variant="secondary" className="gap-1 pr-1 font-normal">
            <span className="max-w-[8rem] truncate">{tag}</span>
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              className="size-5"
              aria-label={t('common.delete')}
              onClick={() => onChange(removeTag(tags, tag))}
            >
              <XIcon className="size-3" />
            </Button>
          </Badge>
        ))}
      </div>
      <Input
        id="card-tags"
        value={draft}
        maxLength={CARD_TAG_MAX}
        placeholder={t('card.tagPlaceholder')}
        onChange={(e) => {
          setDraft(e.target.value)
          if (hint) setHint('')
        }}
        onKeyDown={onKeyDown}
        onBlur={() => commit(draft)}
        aria-describedby={hint ? 'card-tag-hint' : undefined}
      />
      {hint && (
        <p id="card-tag-hint" role="alert" className="text-xs text-destructive">
          {hint}
        </p>
      )}
    </div>
  )
}