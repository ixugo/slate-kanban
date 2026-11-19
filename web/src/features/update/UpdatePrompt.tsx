// 打开网页时问服务端有没有新版本；有，并且用户没有忽略过这个版本，才加载弹窗。
// 查询失败、没有更新都不显示任何东西，也不报错。弹窗本身按需加载，不占首屏体积。

import { useQuery } from '@tanstack/react-query'
import { lazy, Suspense, useState } from 'react'
import { getUpdateInfo } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/queryKeys'
import { isNewer, readIgnored } from './version'

const UpdateDialog = lazy(() => import('./UpdateDialog'))

/** 更新提示：挂在外框里，每次打开网页只查一次（服务端还会缓存 GitHub 的结果） */
export function UpdatePrompt() {
  const [closed, setClosed] = useState(false)
  const { data } = useQuery({
    queryKey: queryKeys.update,
    queryFn: ({ signal }) => getUpdateInfo(signal),
    staleTime: Infinity,
    retry: false,
    refetchOnWindowFocus: false,
  })
  if (closed || !data?.has_update) return null
  const ignored = readIgnored()
  if (ignored !== '' && !isNewer(data.latest, ignored)) return null
  return (
    <Suspense fallback={null}>
      <UpdateDialog info={data} onClose={() => setClosed(true)} />
    </Suspense>
  )
}
