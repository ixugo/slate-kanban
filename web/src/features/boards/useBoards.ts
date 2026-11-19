// 看板列表的查询，以及创建看板的变更。

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createBoard, listBoards } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'

/** 读取全部看板 */
export function useBoards() {
  return useQuery({ queryKey: queryKeys.boards, queryFn: ({ signal }) => listBoards(signal) })
}

/** 创建看板；成功后让看板列表重新取一次，调用方在 onSuccess 里负责跳转 */
export function useCreateBoard() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: createBoard,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.boards }),
  })
}
