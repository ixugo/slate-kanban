// 访问口令相关的查询和操作：读状态、登录。

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getAuthStatus, login } from '@/lib/api/endpoints'
import type { AuthStatus } from '@/lib/api/types'
import { queryKeys } from '@/lib/queryKeys'

/** 已登录的状态：服务端启用了口令，并且当前浏览器通过了 */
const SIGNED_IN: AuthStatus = { required: true, authenticated: true }

/**
 * 读口令状态。不重试也不自动刷新：断网时不能挡住离线只读；
 * 会话过期靠任何接口的 401 触发重新读取（见 lib/queryClient.ts）。
 */
export function useAuthStatus() {
  return useQuery({
    queryKey: queryKeys.auth,
    queryFn: ({ signal }) => getAuthStatus(signal),
    retry: false,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  })
}

/** 用口令登录；成功后把状态改成已登录，并让其他数据重新读取 */
export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (passcode: string) => login(passcode),
    onSuccess: () => {
      queryClient.setQueryData(queryKeys.auth, SIGNED_IN)
      void queryClient.invalidateQueries({ predicate: (query) => query.queryKey[0] !== queryKeys.auth[0] })
    },
  })
}
