// 口令门：服务端启用了访问口令、而当前浏览器还没通过时，只显示登录页，其余页面都不渲染。

import { lazy, Suspense, type ReactNode } from 'react'
import { PageFallback } from '@/app/PageFallback'
import { useAuthStatus } from './useAuth'

/** 登录页只有启用口令时才会用到，按需加载，不占首屏体积 */
const LoginPage = lazy(() => import('./LoginPage'))

/**
 * 包住整个应用。判断规则：只有明确知道「需要口令且没通过」才挡住；
 * 状态还没读到时显示加载条。读状态失败时，由页面显示相应请求的错误。
 */
export function AuthGate({ children }: { children: ReactNode }) {
  const { data, isPending } = useAuthStatus()
  if (isPending) return <PageFallback />
  if (data?.required && !data.authenticated) {
    return (
      <Suspense fallback={<PageFallback />}>
        <LoginPage />
      </Suspense>
    )
  }
  return <>{children}</>
}
