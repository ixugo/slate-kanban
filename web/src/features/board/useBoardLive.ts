// 看板的实时同步：连上实时流，收到事件就让快照重新取一次；断线、reset、看板被删、令牌失效都在这里处理。
// 不在前端逐种事件去改本地数据：自动化规则会连续改好几处，前端重复实现一遍服务端的规则迟早对不上。

import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useEffectEvent, useRef, useState } from 'react'
import { getSnapshot } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { connectBoardStream, type StreamFailure, type StreamState } from '@/lib/stream'
import { clearToken, getToken } from '@/lib/token'

/** 收到事件后等多久再刷新：这段时间里的多条事件合并成一次刷新（一次操作常常连带好几条事件） */
export const REFRESH_COALESCE_MS = 150

interface LiveOptions {
  boardId: number | null
  /** 快照里的事件序号，实时流从它之后开始；快照还没取到时为 undefined，此时不连接 */
  snapshotSeq: number | undefined
  /** 看板已经不存在（实时流返回 404） */
  onGone: () => void
  /** 实时流因其他原因停止，message 是原因 */
  onStopped: (message: string) => void
}

/** 实时流的状态，以及手动重连的函数 */
export interface LiveHandle {
  state: StreamState
  /** 手动重连，用于流已经停止后的"重试"按钮 */
  retry: () => void
}

/**
 * 连接看板的实时流。快照取到之后才连接，之后快照序号变化不会重连（序号只在每次连接开始时用一次）。
 * 返回当前连接状态，页面据此显示"已连接/重连中"的小圆点。
 */
export function useBoardLive({ boardId, snapshotSeq, onGone, onStopped }: LiveOptions): LiveHandle {
  const queryClient = useQueryClient()
  const [state, setState] = useState<StreamState>('connecting')
  const [epoch, setEpoch] = useState(0)
  // reset 之后重新取到的快照序号：下一次连接从它开始，而不是用已经过期的旧序号
  const resumeRef = useRef<number | null>(null)
  const hasSnapshot = snapshotSeq !== undefined

  const readSeq = useEffectEvent(() => snapshotSeq)
  const gone = useEffectEvent(onGone)
  const stopped = useEffectEvent(onStopped)

  useEffect(() => {
    if (boardId === null || !hasSnapshot) return
    let timer: ReturnType<typeof setTimeout> | null = null

    // 合并刷新：快照、卡片详情、活动流一起失效；看板信息变了（改名、删除）列表页也要刷新
    const refresh = (type: string) => {
      if (type.startsWith('board.')) void queryClient.invalidateQueries({ queryKey: queryKeys.boards })
      if (timer) return
      timer = setTimeout(() => {
        timer = null
        void queryClient.invalidateQueries({ queryKey: queryKeys.board(boardId) })
        void queryClient.invalidateQueries({ queryKey: queryKeys.card() })
        void queryClient.invalidateQueries({ queryKey: queryKeys.activity(boardId) })
      }, REFRESH_COALESCE_MS)
    }

    // 续传位置失效：重新取快照，用它的序号重新连接
    const resync = async () => {
      try {
        const snap = await queryClient.fetchQuery({
          queryKey: queryKeys.board(boardId),
          queryFn: ({ signal }) => getSnapshot(boardId, signal),
          staleTime: 0,
        })
        resumeRef.current = snap.seq
        setEpoch((n) => n + 1)
      } catch (err) {
        stopped(err instanceof Error ? err.message : 'resync failed')
      }
    }

    const fatal = (kind: StreamFailure, message: string) => {
      if (kind === 'unauthorized') clearToken()
      else if (kind === 'not_found') gone()
      else stopped(message)
    }

    const after = resumeRef.current ?? readSeq() ?? 0
    resumeRef.current = null
    const stop = connectBoardStream({
      boardId,
      after,
      getToken,
      onEvent: (event) => refresh(event.type),
      onState: setState,
      onReset: () => void resync(),
      onFatal: fatal,
    })
    return () => {
      stop()
      if (timer) clearTimeout(timer)
    }
  }, [boardId, hasSnapshot, epoch, queryClient])

  const retry = useCallback(() => setEpoch((n) => n + 1), [])
  return { state, retry }
}
