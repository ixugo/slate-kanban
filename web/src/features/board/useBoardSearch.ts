// 看板搜索：关键字 300 毫秒防抖，再和点选的筛选词合成一条查询，调用 searchCards，把命中的任务 id 集合交给画布过滤。

import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState } from 'react'
import { searchCards } from '@/lib/api/endpoints'
import { describeError, isAbortError } from '@/lib/errors'
import { FILTER_MAX, SEARCH_LIMIT } from '@/lib/limits'
import { queryKeys } from '@/lib/queryKeys'
import { useTranslation } from '@/i18n/react'
import { joinQuery } from './filters'

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

/**
 * 管理搜索框的状态与查询；boardId 不合法时不发请求。
 * filterTerms 是点选筛选拼出来的词，点一下立刻生效，不防抖；只有输入关键字才防抖。
 */
export function useBoardSearch(boardId: number | null, filterTerms: readonly string[] = []): BoardSearchState {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const [debouncedKeyword, setDebouncedKeyword] = useState('')

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedKeyword(query.trim()), DEBOUNCE_MS)
    return () => clearTimeout(timer)
  }, [query])

  const debounced = joinQuery(debouncedKeyword, filterTerms)
  // 超过服务端的长度上限就不发请求，直接告诉用户，不能悄悄截断：截在半个词上会变成另一个条件
  const tooLong = Array.from(debounced).length > FILTER_MAX
  const enabled = boardId !== null && debounced.length > 0 && !tooLong
  const searchKey = enabled ? [...queryKeys.board(boardId as number), 'search', debounced] : ['search', 'idle']

  const { data, error, isFetching } = useQuery({
    queryKey: searchKey,
    queryFn: ({ signal }) => searchCards(boardId as number, debounced, SEARCH_LIMIT, signal),
    enabled,
    staleTime: 0,
    // 条件变了、新结果还没回来时先用上一次的结果，点一下筛选不会闪一下空白
    placeholderData: keepPreviousData,
  })

  const syntaxError = useMemo(() => {
    if (tooLong) return t('filter.tooLong')
    if (!error || isAbortError(error)) return ''
    const { message, detail } = describeError(error, t)
    return detail || message
  }, [error, t, tooLong])

  const visibleIds = useMemo(() => {
    if (!debounced) return null
    // 第一次结果还没回来时先显示全部任务；出错或条件太长时显示空，让提示文字解释为什么
    if (!data) return error || tooLong ? new Set<number>() : null
    return new Set(data.cards.map((c) => c.id))
  }, [data, debounced, error, tooLong])

  return {
    query,
    setQuery,
    visibleIds,
    truncated: data?.truncated ?? false,
    syntaxError,
    searching: enabled && isFetching,
  }
}