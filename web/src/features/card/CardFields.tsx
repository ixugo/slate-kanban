// 卡片弹窗里的可编辑字段（除描述、评论外即时保存）。

import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column } from '@/lib/api/types'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { CARD_BRANCH_MAX, CARD_TITLE_MAX, CARD_URL_MAX } from '@/lib/limits'
import { toLocalInput } from '@/lib/format'
import { DependencyPicker } from './DependencyPicker'
import { normalizeCardUrl } from './validateUrl'
import { toast } from '@/lib/toast'

interface CardFieldsProps {
  card: CardDetail
  columns: Column[]
  depOptions: CardDetail['dependencies']
  version: number
  onPatch: (patch: Record<string, unknown>) => void
  onColumn: (columnId: number) => void
}

/** 标题、列、优先级等字段 */
export function CardFields({ card, columns, depOptions, version, onPatch, onColumn }: CardFieldsProps) {
  const { t } = useTranslation()
  const priorities = ['none', 'low', 'medium', 'high'] as const

  const saveTitle = (title: string) => onPatch({ title: title.trim().slice(0, CARD_TITLE_MAX), version })

  const saveUrl = (url: string) => {
    const norm = normalizeCardUrl(url)
    if (!norm.ok) {
      toast.error(t('card.urlInvalid'))
      return
    }
    onPatch({ url: norm.value.slice(0, CARD_URL_MAX), version })
  }

  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <div className="space-y-1 sm:col-span-2">
        <Label htmlFor="card-title">{t('card.field.title')}</Label>
        <Input
          id="card-title"
          defaultValue={card.title}
          maxLength={CARD_TITLE_MAX}
          onBlur={(e) => {
            if (e.target.value.trim() !== card.title) saveTitle(e.target.value)
          }}
        />
      </div>
      <div className="space-y-1">
        <Label>{t('card.field.column')}</Label>
        <Select value={String(card.column_id)} onValueChange={(v) => onColumn(Number(v))}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {columns.map((col) => (
              <SelectItem key={col.id} value={String(col.id)}>
                {col.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1">
        <Label>{t('card.field.priority')}</Label>
        <Select
          value={String(card.priority)}
          onValueChange={(v) => onPatch({ priority: Number(v), version })}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {priorities.map((p, i) => (
              <SelectItem key={p} value={String(i)}>
                {t(`board.priority.${p}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1">
        <Label htmlFor="card-due">{t('card.field.due')}</Label>
        <Input
          id="card-due"
          type="datetime-local"
          defaultValue={toLocalInput(card.due_at)}
          onBlur={(e) => onPatch({ due_at: e.target.value === '' ? '' : e.target.value, version })}
        />
      </div>
      <div className="flex items-center gap-2">
        <Switch
          id="card-human"
          checked={card.human_only}
          onCheckedChange={(v) => onPatch({ human_only: v, version })}
        />
        <Label htmlFor="card-human">{t('card.field.humanOnly')}</Label>
      </div>
      <div className="space-y-1">
        <Label htmlFor="card-branch">{t('card.field.branch')}</Label>
        <Input
          id="card-branch"
          defaultValue={card.branch}
          maxLength={CARD_BRANCH_MAX}
          onBlur={(e) => {
            if (e.target.value !== card.branch) onPatch({ branch: e.target.value.trim().slice(0, CARD_BRANCH_MAX), version })
          }}
        />
      </div>
      <div className="space-y-1 sm:col-span-2">
        <Label htmlFor="card-url">{t('card.field.url')}</Label>
        <Input
          id="card-url"
          defaultValue={card.url}
          maxLength={CARD_URL_MAX}
          onBlur={(e) => {
            if (e.target.value !== card.url) saveUrl(e.target.value)
          }}
        />
      </div>
      <div className="space-y-1 sm:col-span-2">
        <Label>{t('card.field.deps')}</Label>
        <DependencyPicker
          cardId={card.id}
          selected={card.waits_on}
          options={depOptions}
          onChange={(waits_on) => onPatch({ waits_on, version })}
        />
      </div>
    </div>
  )
}