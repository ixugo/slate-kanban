// 看板的实时同步：用浏览器自带的 EventSource 连上实时流，收到事件就让快照重新取一次。
// 服务端不需要登录，所以不用自己解析 SSE、也不用自己写重连：EventSource 断线会自动重连，
// 并带上最后收到的事件序号（Last-Event-ID），服务端从这个序号接着补发漏掉的事件。
// 不在前端逐种事件去改本地数据：自动化规则会连续改好几处，前端重复实现一遍服务端的规则迟早对不上。

import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useEffectEvent, useState } from 'react'
import { API_PREFIX, ApiError } from '@/lib/api/client'
import { getSnapshot } from '@/lib/api/endpoints'
import type { Snapshot } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'

/** 收到事件后等多久再刷新：这段时间里的多条事件合并成一次刷新（一次操作常常连带好几条事件） */
export const REFRESH_COALESCE_MS = 150

/** 浏览器放弃重连（比如服务端返回了错误状态码）后，等多久再自己重新连一次 */
export const RECONNECT_DELAY_MS = 5000

/** 实时流的状态：正在连、已连上、断开 */
export type LiveState = 'connecting' | 'live' | 'offline'

interface LiveOptions {
  /** 看板编号；为 null（地址不合法）时不连接 */
  boardId: number | null
  /** 确认看板已经不存在（快照返回 404）时调用 */
  onGone: () => void
}

/** 实时流的状态，以及手动重连的函数 */
export interface LiveHandle {
  state: LiveState
  /** 手动重连 */
  retry: () => void
}

// 解析服务端事件里的 JSON；格式不对返回 null，调用方照常刷新，不因为一条坏数据漏掉更新。
function parseData(raw: unknown): Record<string, unknown> | null {
  if (typeof raw !== 'string') return null
  try {
    const parsed: unknown = JSON.parse(raw)
    return typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, unknown>) : null
  } catch {
    return null
  }
}

/**
 * 连接看板的实时流，返回当前连接状态，页面据此显示"已连接/重连中"的小圆点。
 * 连上之后服务端先发 ready（带最新序号）：比缓存里的快照新，说明"取快照"到"连上"之间有更新，要补刷一次。
 */
export function useBoardLive({ boardId, onGone }: LiveOptions): LiveHandle {
  const queryClient = useQueryClient()
  const [epoch, setEpoch] = useState(0)
  // 状态连同"哪一次连接"一起记：换看板或重连时 key 变了，状态自然回到"连接中"，不用在 effect 里同步改状态
  const key = `${boardId}:${epoch}`
  const [live, setLive] = useState<{ key: string; state: LiveState }>({ key, state: 'connecting' })
  const state = live.key === key ? live.state : 'connecting'
  const gone = useEffectEvent(onGone)

  useEffect(() => {
    if (boardId === null) return
    let active = true
    let refreshTimer: ReturnType<typeof setTimeout> | null = null
    let retryTimer: ReturnType<typeof setTimeout> | null = null
    const source = new EventSource(`${API_PREFIX}/boards/${boardId}/stream`)
    const setState = (next: LiveState) => setLive({ key, state: next })

    // 立即刷新：快照和卡片详情一起失效
    const flush = () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.card() })
    }

    // 合并刷新：一小段时间内的多条事件只刷新一次；看板本身变了（改名、删除）列表页也要刷新
    const refresh = (type: string) => {
      if (type.startsWith('board.')) void queryClient.invalidateQueries({ queryKey: queryKeys.boards })
      if (refreshTimer) return
      refreshTimer = setTimeout(() => {
        refreshTimer = null
        flush()
      }, REFRESH_COALESCE_MS)
    }

    // 流断开后确认看板还在不在：404 说明被删了；其他错误（网络、服务重启）等一会儿自己重连
    const checkAlive = async () => {
      try {
        await queryClient.fetchQuery({
          queryKey: queryKeys.board(boardId),
          queryFn: ({ signal }) => getSnapshot(boardId, signal),
          staleTime: 0,
        })
      } catch (err) {
        if (!active) return
        if (err instanceof ApiError && err.status === 404) {
          gone()
          return
        }
      }
      if (active) retryTimer = setTimeout(() => setEpoch((n) => n + 1), RECONNECT_DELAY_MS)
    }

    source.addEventListener('ready', (e) => {
      setState('live')
      const latest = parseData((e as MessageEvent).data)?.latest
      const cached = queryClient.getQueryData<Snapshot>(queryKeys.board(boardId))
      if (typeof latest !== 'number' || !cached || latest > cached.seq) refresh('')
    })
    source.addEventListener('board', (e) => {
      const type = parseData((e as MessageEvent).data)?.type
      refresh(typeof type === 'string' ? type : '')
    })
    // 续传位置失效：关掉旧连接，刷新后重新连（新连接从最新位置开始）
    source.addEventListener('reset', () => {
      source.close()
      flush()
      setEpoch((n) => n + 1)
    })
    // error 既是服务端发来的事件（流开始后出错），也是浏览器的连接错误：前者带数据，后者是普通 Event
    source.addEventListener('error', (e) => {
      if (e instanceof MessageEvent) {
        source.close()
        setState('offline')
        void checkAlive()
        return
      }
      if (source.readyState === EventSource.CLOSED) {
        setState('offline')
        void checkAlive()
      } else {
        setState('connecting')
      }
    })

    return () => {
      active = false
      source.close()
      if (refreshTimer) clearTimeout(refreshTimer)
      if (retryTimer) clearTimeout(retryTimer)
    }
  }, [boardId, epoch, key, queryClient])

  const retry = useCallback(() => setEpoch((n) => n + 1), [])
  return { state, retry }
}
