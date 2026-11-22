import { useState } from 'react'
import { queryOptions, useQuery, useQueryClient } from '@tanstack/react-query'
import { getCardByNumber } from '@/lib/api/endpoints'
import type { Card, CardView } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'
import { CardReference } from './CardReference'

interface CardNumberReferenceProps {
  boardId: number
  number: number
  label: string
  card?: CardView
  onOpen: (card: Card | undefined) => void
}

/** 归档任务不在看板快照中，首次预览或打开时按同板编号读取。 */
export function CardNumberReference({ boardId, number, label, card, onOpen }: CardNumberReferenceProps) {
  const [preview, setPreview] = useState(false)
  const queryClient = useQueryClient()
  const options = queryOptions({
    queryKey: queryKeys.cardByNumber(boardId, number),
    queryFn: ({ signal }) => getCardByNumber(boardId, number, signal),
    retry: false,
  })
  const { data } = useQuery({ ...options, enabled: !card && preview })
  const target = card ?? data
  return (
    <CardReference
      label={label}
      card={target}
      onPreviewChange={card ? undefined : setPreview}
      onOpen={() => {
        if (target) onOpen(target)
        else void queryClient.fetchQuery(options).then(onOpen, () => onOpen(undefined))
      }}
    />
  )
}
