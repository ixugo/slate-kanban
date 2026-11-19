// 卡片的标签选择器（Trello 式）：搜索框、带颜色的标签列表（勾选即加到卡片上）、每项的编辑按钮、新建标签。
// 标签库里每个标签是一个颜色加一段文字（文字可以为空）；卡片按主键引用标签。
// 搜索框里输入文字后回车：库里有同文字的标签就勾选它，没有就新建一个无颜色的标签并加到卡片上。

import { useState, type KeyboardEvent } from 'react'
import { CheckIcon, PencilIcon, PlusIcon } from 'lucide-react'
import { useTranslation, type TFunction } from '@/i18n/react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Tag, TagInput } from '@/lib/api/types'
import { CARD_TAGS_MAX } from '@/lib/limits'
import { labelStyle } from '@/lib/labelColors'
import { cn } from '@/lib/utils'
import { LabelEditor, type LabelDraft } from './LabelEditor'
import { tagName, tagText } from './tagName'
import { useLabels } from './useLabels'
import { useSaveLabels } from './useSaveLabels'

interface LabelPickerProps {
  /** 这张卡片现有的标签主键 */
  tagIds: number[]
  onChange: (tagIds: number[]) => void
}

type View = { kind: 'list' } | { kind: 'edit'; tag: Tag | null }

/** 标签库的一项转成保存入参 */
const toInput = (tag: Tag): TagInput => ({ id: tag.id, text: tag.text, color: tag.color })

/** 标签的原文字或译文（都转小写）有一个满足条件就算命中；起步标签搜原文和译文都能找到 */
const matchText = (tag: Tag, t: TFunction, test: (lowerText: string) => boolean): boolean =>
  test(tag.text.toLowerCase()) || test(tagText(tag.text, t).toLowerCase())

/** 标签选择器：列表页和编辑页两个视图 */
export function LabelPicker({ tagIds, onChange }: LabelPickerProps) {
  const { t } = useTranslation()
  const { boardId, tags } = useLabels()
  const save = useSaveLabels(boardId)
  const [view, setView] = useState<View>({ kind: 'list' })
  const [query, setQuery] = useState('')
  const [hint, setHint] = useState('')

  const needle = query.trim().toLowerCase()
  const visible = needle ? tags.filter((tag) => matchText(tag, t, (text) => text.includes(needle))) : tags
  const has = (id: number) => tagIds.includes(id)

  /** 勾选或取消一个标签；卡片已经满了就提示 */
  const toggle = (id: number) => {
    if (has(id)) {
      setHint('')
      onChange(tagIds.filter((x) => x !== id))
    } else if (tagIds.length >= CARD_TAGS_MAX) {
      setHint(t('card.tagLimit', { max: String(CARD_TAGS_MAX) }))
    } else {
      setHint('')
      onChange([...tagIds, id])
    }
  }

  /** 把一个标签追加进标签库（颜色可为空）并加到这张卡片上；保存成功后回到列表 */
  const createAndAttach = (text: string, color: string, onDone?: () => void) => {
    if (tagIds.length >= CARD_TAGS_MAX) {
      setHint(t('card.tagLimit', { max: String(CARD_TAGS_MAX) }))
      return
    }
    save.mutate([...tags.map(toInput), { text, color }], {
      onSuccess: (board) => {
        const created = board.tags.find((tag) => !tags.some((old) => old.id === tag.id))
        if (created) onChange([...tagIds, created.id])
        onDone?.()
      },
    })
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter' && e.key !== ',') return
    e.preventDefault()
    const text = query.trim()
    if (!text) return
    const same = tags.find((tag) => matchText(tag, t, (name) => name === text.toLowerCase()))
    if (same) {
      if (!has(same.id)) toggle(same.id)
      setQuery('')
    } else {
      createAndAttach(text, '', () => setQuery(''))
    }
  }

  /** 保存编辑页：新建的追加到库并加到这张卡片上，已有的按主键改文字和颜色 */
  const onSave = (draft: LabelDraft) => {
    if (draft.id === 0) {
      createAndAttach(draft.text, draft.color, () => setView({ kind: 'list' }))
      return
    }
    const next = tags.map((tag) => (tag.id === draft.id ? { ...toInput(tag), text: draft.text, color: draft.color } : toInput(tag)))
    save.mutate(next, { onSuccess: () => setView({ kind: 'list' }) })
  }

  const onDelete = (id: number) => {
    save.mutate(tags.filter((tag) => tag.id !== id).map(toInput), { onSuccess: () => setView({ kind: 'list' }) })
  }

  if (view.kind === 'edit') {
    return <LabelEditor tag={view.tag} busy={save.isPending} onBack={() => setView({ kind: 'list' })} onSave={onSave} onDelete={onDelete} />
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
        {visible.map((tag) => (
          <li key={tag.id} className="flex items-center gap-1.5">
            <button
              type="button"
              role="checkbox"
              aria-checked={has(tag.id)}
              aria-label={tagName(tag, t)}
              style={labelStyle(tag.color)}
              className={cn(
                'flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md px-2.5 text-left text-ui font-medium outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 pointer-coarse:h-10',
                !tag.color && 'bg-secondary text-foreground',
              )}
              onClick={() => toggle(tag.id)}
            >
              <span className="min-w-0 flex-1 truncate">{tagText(tag.text, t)}</span>
              {has(tag.id) && <CheckIcon className="size-4 shrink-0" aria-hidden />}
            </button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t('label.edit', { name: tagName(tag, t) })}
              onClick={() => setView({ kind: 'edit', tag })}
            >
              <PencilIcon />
            </Button>
          </li>
        ))}
      </ul>
      <Button type="button" variant="secondary" size="sm" onClick={() => setView({ kind: 'edit', tag: null })}>
        <PlusIcon />
        {t('label.create')}
      </Button>
    </div>
  )
}
