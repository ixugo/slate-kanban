// 路由表：不需要登录，所有页面都在外框里面。看板页、设置页、接入页体积较大，按路由懒加载，首屏只带看板列表。

import { lazy, Suspense } from 'react'
import { Route, Routes } from 'react-router-dom'
import { BoardListPage } from '@/features/boards/BoardListPage'
import { AppShell } from './AppShell'
import { NotFoundPage } from './NotFoundPage'
import { PageFallback } from './PageFallback'

const BoardPage = lazy(() => import('@/features/board/BoardPage'))
const SettingsPage = lazy(() => import('@/features/settings/SettingsPage'))
const AgentsPage = lazy(() => import('@/features/agents/AgentsPage'))

/** 全部页面的路由 */
export function AppRoutes() {
  return (
    <Suspense fallback={<PageFallback />}>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<BoardListPage />} />
          <Route path="b/:boardId" element={<BoardPage />} />
          <Route path="b/:boardId/settings" element={<SettingsPage />} />
          <Route path="agents" element={<AgentsPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </Suspense>
  )
}
