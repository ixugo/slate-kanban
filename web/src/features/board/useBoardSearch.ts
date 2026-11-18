// 看板搜索：300 毫秒防抖后调用 searchCards，把命中的卡片 id 集合交给画布过滤。

import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { searchCards } from '@/lib/api/endpoints'
import { describeError, isAbortError } from '@/lib/errors'
import { FILTER_MAX, SEARCH_LIMIT } from '@/lib/limits'
import { queryKeys } from '@/lib/queryKeys'
import { useTranslation } from '@/i18n/react'

const DEBOUNCE_MS = 300

/** 搜索状态：输入值、命中 id、是否被截断、语法错误文案 */
export interface BoardSearchState {
  query: string
  setQuery: (q: string) => void
  /** 有搜索词时为命中 id，否则 null 表示不过滤 */
  visibleIds: ReadonlySet<number> | null
  truncated: boolean
  /** 服务端 400 时的具体原因（中文），空表示没有错误 */
  syntaxError: string
  searching: boolean
}

/** 管理搜索框的状态与查询；boardId 不合法时不发请求 */
export function useBoardSearch(boardId: number | null): BoardSearchState {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')

  useEffect(() => {
    const trimmed = query.trim().slice(0, FILTER_MAX)
    const timer = setTimeout(() => setDebounced(trimmed), DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  const enabled = boardId !== null && debounced.length > 0
  const searchKey = enabled ? [...queryKeys.board(boardId as number), 'search', debounced] : ['search', 'idle']

  const { data, error, isFetching } = useQuery({
    queryKey: searchKey,
    queryFn: ({ signal }) => searchCards(boardId as number, debounced, SEARCH_LIMIT, signal),
    enabled,
    staleTime: 0,
  })

  const syntaxError = useMemo(() => {
    if (!error || isAbortError(error)) return ''
    const { message, detail } = describeError(error, t)
    return detail || message
  }, [error, t])

  const visibleIds = useMemo(() => {
    if (!debounced) return null
    if (!data) return new Set<number>()
    return new Set(data.cards.map((c) => c.id))
  }, [data, debounced])

  return {
    query,
    setQuery,
    visibleIds,
    truncated: data?.truncated ?? false,
    syntaxError,
    searching: enabled && isFetching,
  }
}