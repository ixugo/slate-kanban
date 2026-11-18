// 编辑或新建一个标签：预览、标题、30 种颜色、移除颜色、保存和删除。

import { useEffect, useRef, useState } from 'react'
import { ArrowLeftIcon, CheckIcon, XIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label as FieldLabel } from '@/components/ui/label'
import { CARD_TAG_MAX } from '@/lib/limits'
import { LABEL_HUES, LABEL_SHADES, labelStyle } from '@/lib/labelColors'
import { cn } from '@/lib/utils'
import type { LabelOption } from './useLabels'

/** 编辑中的标签：id 为空且 name 为空表示新建；id 为空但 name 不空表示给只在卡片上用过的标签登记颜色 */
export interface LabelDraft {
  id: string
  name: string
  color: string
}

interface LabelEditorProps {
  /** 被编辑的标签；null 表示新建 */
  option: LabelOption | null
  busy: boolean
  onBack: () => void
  onSave: (draft: LabelDraft) => void
  /** 只有已登记进库的标签才能删除 */
  onDelete: (id: string) => void
}

/** 标签编辑页 */
export function LabelEditor({ option, busy, onBack, onSave, onDelete }: LabelEditorProps) {
  const { t } = useTranslation()
  const [name, setName] = useState(option?.name ?? '')
  const [color, setColor] = useState(option?.color ?? '')
  const registered = Boolean(option?.id)
  // 没登记进库的标签在别的卡片上也用着，改名只会改到这一张，所以只允许给它配颜色
  const nameLocked = Boolean(option) && !registered
  const trimmed = name.trim()
  const nameRef = useRef<HTMLInputElement>(null)

  // 新建时直接聚焦标题；不用 autoFocus，因为它会把弹层滚到输入框，顶上的标题栏和返回按钮就看不见了
  useEffect(() => {
    if (!nameLocked) nameRef.current?.focus({ preventScroll: true })
  }, [nameLocked])

  const submit = () => {
    if (trimmed) onSave({ id: option?.id ?? '', name: trimmed, color })
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="relative flex items-center justify-center">
        <Button type="button" variant="ghost" size="icon-xs" className="absolute left-0" aria-label={t('label.back')} onClick={onBack}>
          <ArrowLeftIcon />
        </Button>
        <h4 className="text-ui font-semibold">{option ? t('label.editTitle') : t('label.createTitle')}</h4>
      </div>
      <div className="flex items-center justify-center rounded-xl bg-secondary/60 p-2.5">
        <span
          style={labelStyle(color)}
          className={cn('block h-8 w-full truncate rounded-lg px-3 text-center text-ui leading-8 font-semibold', !color && 'bg-muted text-muted-foreground')}
        >
          {trimmed}
        </span>
      </div>
      <div className="space-y-1.5">
        <FieldLabel htmlFor="label-name" className="text-muted-foreground">
          {t('label.name')}
        </FieldLabel>
        <Input
          id="label-name"
          ref={nameRef}
          value={name}
          maxLength={CARD_TAG_MAX}
          readOnly={nameLocked}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
      <div className="space-y-1.5">
        <p className="text-xs font-medium text-muted-foreground">{t('label.color')}</p>
        <div role="radiogroup" aria-label={t('label.color')} className="grid grid-cols-5 gap-1.5">
          {LABEL_SHADES.flatMap((shade) =>
            LABEL_HUES.map((hue) => {
              const id = `${hue}-${shade}`
              const name = t('label.colorName', { hue: t(`label.hues.${hue}`), shade: t(`label.shades.${shade === 1 ? 'dark' : shade === 2 ? 'medium' : 'light'}`) })
              return (
                <button
                  key={id}
                  type="button"
                  role="radio"
                  aria-checked={color === id}
                  aria-label={name}
                  title={name}
                  style={labelStyle(id)}
                  className="flex h-7 items-center justify-center rounded-md outline-none transition-transform hover:scale-105 focus-visible:ring-[3px] focus-visible:ring-ring/50 pointer-coarse:h-10"
                  onClick={() => setColor(id)}
                >
                  {color === id && <CheckIcon className="size-4" aria-hidden />}
                </button>
              )
            }),
          )}
        </div>
        <Button type="button" variant="secondary" size="sm" className="w-full" disabled={!color} onClick={() => setColor('')}>
          <XIcon />
          {t('label.removeColor')}
        </Button>
      </div>
      <div className="flex items-center justify-between gap-2 border-t border-border pt-3">
        <Button type="button" size="sm" disabled={busy || !trimmed} onClick={submit}>
          {t('common.save')}
        </Button>
        {registered && (
          <Button type="button" size="sm" variant="destructive" disabled={busy} onClick={() => onDelete(option!.id)}>
            {t('common.delete')}
          </Button>
        )}
      </div>
    </div>
  )
}
