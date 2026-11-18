// 卡片弹窗的属性区：列、优先级、标签、截止等可编辑属性；宽一点时分两列，窄屏单列。

import { useTranslation } from '@/i18n/react'
import type { CardDetail, Column } from '@/lib/api/types'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { CARD_BRANCH_MAX, CARD_URL_MAX } from '@/lib/limits'
import { toLocalInput } from '@/lib/format'
import { DependencyPicker } from './DependencyPicker'
import { TagEditor } from './TagEditor'
import { normalizeCardUrl } from './validateUrl'
import { toast } from '@/lib/toast'
import { useTemplateText } from '@/lib/useTemplateText'

interface CardPropertiesProps {
  card: CardDetail
  columns: Column[]
  depOptions: CardDetail['dependencies']
  version: number
  onPatch: (patch: Record<string, unknown>) => void
  onColumn: (columnId: number) => void
}

/** 属性表单（不含标题、描述、评论） */
export function CardProperties({ card, columns, depOptions, version, onPatch, onColumn }: CardPropertiesProps) {
  const { t } = useTranslation()
  const { columnName } = useTemplateText()
  const priorities = ['none', 'low', 'medium', 'high'] as const

  const saveUrl = (url: string) => {
    const norm = normalizeCardUrl(url)
    if (!norm.ok) {
      toast.error(t('card.urlInvalid'))
      return
    }
    onPatch({ url: norm.value.slice(0, CARD_URL_MAX), version })
  }

  return (
    <div className="grid gap-3.5 rounded-2xl bg-secondary/50 p-4 sm:grid-cols-2">
      <div className="space-y-1.5">
        <Label className="text-muted-foreground">{t('card.field.column')}</Label>
        <Select value={String(card.column_id)} onValueChange={(v) => onColumn(Number(v))}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {columns.map((col) => (
              <SelectItem key={col.id} value={String(col.id)}>
                {columnName(col.name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-1.5">
        <Label className="text-muted-foreground">{t('card.field.priority')}</Label>
        <Select value={String(card.priority)} onValueChange={(v) => onPatch({ priority: Number(v), version })}>
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
      <div className="sm:col-span-2">
        <TagEditor tags={card.tags} onChange={(tags) => onPatch({ tags, version })} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="card-due" className="text-muted-foreground">{t('card.field.due')}</Label>
        <Input
          id="card-due"
          type="datetime-local"
          defaultValue={toLocalInput(card.due_at)}
          onBlur={(e) => onPatch({ due_at: e.target.value === '' ? '' : e.target.value, version })}
        />
      </div>
      <div className="flex items-center justify-between gap-2 sm:self-end sm:pb-1.5">
        <Label htmlFor="card-human" className="text-muted-foreground">{t('card.field.humanOnly')}</Label>
        <Switch id="card-human" checked={card.human_only} onCheckedChange={(v) => onPatch({ human_only: v, version })} />
      </div>
      <div className="space-y-1.5 sm:col-span-2">
        <Label className="text-muted-foreground">{t('card.field.deps')}</Label>
        <DependencyPicker
          cardId={card.id}
          selected={card.waits_on}
          options={depOptions}
          onChange={(waits_on) => onPatch({ waits_on, version })}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="card-branch" className="text-muted-foreground">{t('card.field.branch')}</Label>
        <Input
          id="card-branch"
          defaultValue={card.branch}
          maxLength={CARD_BRANCH_MAX}
          onBlur={(e) => {
            if (e.target.value !== card.branch) onPatch({ branch: e.target.value.trim().slice(0, CARD_BRANCH_MAX), version })
          }}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="card-url" className="text-muted-foreground">{t('card.field.url')}</Label>
        <Input
          id="card-url"
          defaultValue={card.url}
          maxLength={CARD_URL_MAX}
          onBlur={(e) => {
            if (e.target.value !== card.url) saveUrl(e.target.value)
          }}
        />
      </div>
    </div>
  )
}