import { GitMergeIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import type { Card } from '@/lib/api/types'
import { cn } from '@/lib/utils'

interface CardMergeBadgeProps {
  card: Pick<Card, 'key' | 'branch' | 'merged_commits'>
  compact?: boolean
}

export function CardMergeBadge({ card, compact = false }: CardMergeBadgeProps) {
  const { t } = useTranslation()
  const hashes = card.merged_commits ?? []
  const merged = hashes.length > 0
  if (!merged && !card.branch.startsWith(`slate/${card.key.toLowerCase()}-`)) return null

  const label = t(merged ? 'card.field.merged' : 'card.field.mergePending')
  return (
    <span
      className={cn(
        'inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-md px-1.5 font-sans font-medium',
        compact ? 'py-0.5 text-[0.6875rem]' : 'h-8 px-2.5 text-sm',
        merged ? 'bg-accent text-accent-foreground' : 'bg-muted text-muted-foreground',
      )}
      title={merged ? `${label} · ${hashes.join(', ')}` : undefined}
    >
      <GitMergeIcon className={compact ? 'size-3' : 'size-4'} aria-hidden />
      {label}
    </span>
  )
}
