// 卡片的标签选择器（Trello 式）：搜索框、带颜色的标签列表（勾选即加到卡片上）、每项的编辑按钮、新建标签。
// 搜索框里直接回车会把输入的文字当普通标签加到卡片上（不进标签库，没有颜色，之后可以给它配颜色）。

import { useState, type KeyboardEvent } from 'react'
import { CheckIcon, PencilIcon, PlusIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { CARD_TAG_MAX, CARD_TAGS_MAX } from '@/lib/limits'
import { labelStyle } from '@/lib/labelColors'
import { cn } from '@/lib/utils'
import { applyTag, removeTag } from '@/features/card/tagList'
import { LabelEditor, type LabelDraft } from './LabelEditor'
import { useLabels, type LabelOption } from './useLabels'
import { useSaveLabels } from './useSaveLabels'

interface LabelPickerProps {
  /** 这张卡片现有的标签 */
  tags: string[]
  onChange: (tags: string[]) => void
}

type View = { kind: 'list' } | { kind: 'edit'; option: LabelOption | null }

/** 标签选择器：列表页和编辑页两个视图 */
export function LabelPicker({ tags, onChange }: LabelPickerProps) {
  const { t } = useTranslation()
  const { boardId, labels, options } = useLabels()
  const save = useSaveLabels(boardId)
  const [view, setView] = useState<View>({ kind: 'list' })
  const [query, setQuery] = useState('')
  const [hint, setHint] = useState('')

  // 卡片上有、但标签库和别的卡片都没有的标签（刚用回车加的）也要能在列表里看到并取消
  const known = new Set(options.map((o) => o.name.toLowerCase()))
  const shown = [...options, ...tags.filter((x) => !known.has(x.toLowerCase())).map((name) => ({ id: '', name, color: '' }))]
  const needle = query.trim().toLowerCase()
  const visible = needle ? shown.filter((o) => o.name.toLowerCase().includes(needle)) : shown
  const has = (name: string) => tags.some((x) => x.toLowerCase() === name.toLowerCase())

  const add = (raw: string) => {
    const result = applyTag(tags, raw)
    if (!result.ok) {
      if (result.reason === 'limit') setHint(t('card.tagLimit', { max: String(CARD_TAGS_MAX) }))
      else if (result.reason === 'tooLong') setHint(t('card.tagTooLong', { max: String(CARD_TAG_MAX) }))
      else setHint('')
      return false
    }
    setHint('')
    onChange(result.tags)
    return true
  }

  const toggle = (name: string) => {
    if (has(name)) {
      setHint('')
      onChange(removeTag(tags, tags.find((x) => x.toLowerCase() === name.toLowerCase()) ?? name))
    } else add(name)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter' && e.key !== ',') return
    e.preventDefault()
    if (add(query)) setQuery('')
  }

  const toInputs = (list: typeof labels) => list.map((l) => ({ id: l.id, name: l.name, color: l.color }))

  /** 保存编辑页：新建或登记颜色就追加/更新库，已登记的按编号改名改色；新建的标签顺手加到这张卡片上 */
  const onSave = (draft: LabelDraft) => {
    const registered = draft.id !== ''
    const current = toInputs(labels)
    const next = registered
      ? current.map((l) => (l.id === draft.id ? { ...l, name: draft.name, color: draft.color } : l))
      : [...current.filter((l) => l.name.toLowerCase() !== draft.name.toLowerCase()), { name: draft.name, color: draft.color }]
    const creating = view.kind === 'edit' && view.option === null
    save.mutate(next, {
      onSuccess: () => {
        if (creating && !has(draft.name)) add(draft.name)
        setView({ kind: 'list' })
      },
    })
  }

  const onDelete = (id: string) => {
    save.mutate(toInputs(labels).filter((l) => l.id !== id), { onSuccess: () => setView({ kind: 'list' }) })
  }

  if (view.kind === 'edit') {
    return <LabelEditor option={view.option} busy={save.isPending} onBack={() => setView({ kind: 'list' })} onSave={onSave} onDelete={onDelete} />
  }

  return (
    <div className="flex flex-col gap-2.5">
      <h4 className="text-center text-ui font-semibold">{t('label.title')}</h4>
      <Input
        aria-label={t('card.field.tags')}
        placeholder={t('label.search')}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={onKeyDown}
      />
      {hint && (
        <p role="alert" className="text-xs text-destructive">
          {hint}
        </p>
      )}
      <ul className="flex max-h-64 flex-col gap-1.5 overflow-y-auto">
        {visible.length === 0 && <li className="px-1 py-2 text-center text-xs text-muted-foreground">{t('label.empty')}</li>}
        {visible.map((option) => (
          <li key={option.id || option.name} className="flex items-center gap-1.5">
            <button
              type="button"
              role="checkbox"
              aria-checked={has(option.name)}
              style={labelStyle(option.color)}
              className={cn(
                'flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md px-2.5 text-left text-ui font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 pointer-coarse:h-10',
                !option.color && 'bg-secondary text-foreground',
              )}
              onClick={() => toggle(option.name)}
            >
              <span className="min-w-0 flex-1 truncate">{option.name}</span>
              {has(option.name) && <CheckIcon className="size-4 shrink-0" aria-hidden />}
            </button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t('label.edit', { name: option.name })}
              onClick={() => setView({ kind: 'edit', option })}
            >
              <PencilIcon />
            </Button>
          </li>
        ))}
      </ul>
      <Button type="button" variant="secondary" size="sm" onClick={() => setView({ kind: 'edit', option: null })}>
        <PlusIcon />
        {t('label.create')}
      </Button>
    </div>
  )
}
