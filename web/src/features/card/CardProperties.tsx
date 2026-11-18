// 卡片弹窗的属性区：标题下面一排紧凑的优先级、标签、截止、仅人工（对齐 Trello 的做法），
// 以及描述下面的依赖、分支、链接。

import { useTranslation } from '@/i18n/react'
import type { CardDetail } from '@/lib/api/types'
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

interface CardPatchProps {
  card: CardDetail
  onPatch: (patch: Record<string, unknown>) => void
}

/** 标题下面的一排属性：每项一个小标题加一个紧凑控件，放不下就换行 */
export function CardMeta({ card, onPatch }: CardPatchProps) {
  const { t } = useTranslation()
  const priorities = ['none', 'low', 'medium', 'high'] as const
  const version = card.version

  return (
    <div className="flex flex-wrap items-start gap-x-5 gap-y-4">
      <div className="space-y-1.5">
        <Label className="text-muted-foreground">{t('card.field.priority')}</Label>
        <Select value={String(card.priority)} onValueChange={(v) => onPatch({ priority: Number(v), version })}>
          <SelectTrigger size="sm" className="min-w-24">
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
      <div className="min-w-48 flex-1 basis-56">
        <TagEditor tags={card.tags} onChange={(tags) => onPatch({ tags, version })} />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="card-due" className="text-muted-foreground">{t('card.field.due')}</Label>
        <Input
          id="card-due"
          type="datetime-local"
          className="h-8 w-52"
          defaultValue={toLocalInput(card.due_at)}
          onBlur={(e) => onPatch({ due_at: e.target.value === '' ? '' : e.target.value, version })}
        />
      </div>
      <div className="space-y-1.5">
        <Label htmlFor="card-human" className="text-muted-foreground">{t('card.field.humanOnly')}</Label>
        <div className="flex h-8 items-center">
          <Switch id="card-human" checked={card.human_only} onCheckedChange={(v) => onPatch({ human_only: v, version })} />
        </div>
      </div>
    </div>
  )
}

/** 描述下面的关联信息：依赖、分支、链接 */
export function CardLinks({ card, depOptions, onPatch }: CardPatchProps & { depOptions: CardDetail['dependencies'] }) {
  const { t } = useTranslation()
  const version = card.version

  const saveUrl = (url: string) => {
    const norm = normalizeCardUrl(url)
    if (!norm.ok) {
      toast.error(t('card.urlInvalid'))
      return
    }
    onPatch({ url: norm.value.slice(0, CARD_URL_MAX), version })
  }

  return (
    <div className="grid gap-3.5 sm:grid-cols-2">
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
