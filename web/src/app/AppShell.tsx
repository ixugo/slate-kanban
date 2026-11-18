// 所有页面共用的外框：顶栏（标志、看板切换、agent、语言、主题）加页面内容区。

import { BotIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet, useMatch } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { BoardSwitcher } from './BoardSwitcher'
import { LanguageMenu } from './LanguageMenu'
import { Logo } from './Logo'
import { ThemeMenu } from './ThemeMenu'

/** 外框组件；内容区高度撑满剩下的屏幕，看板页要靠它做横向滚动的画布 */
export function AppShell() {
  const { t } = useTranslation()
  const onAgents = useMatch('/agents') !== null
  return (
    <div className="flex h-dvh flex-col">
      <a
        href="#main"
        className="sr-only z-50 bg-background px-3 py-2 text-xs focus:not-sr-only focus:absolute focus:top-2 focus:left-2"
      >
        {t('nav.skipToContent')}
      </a>
      <header className="flex h-12 shrink-0 items-center gap-1 border-b bg-background px-2 sm:px-3">
        <Link to="/" className="mr-1 flex items-center gap-2 px-1 font-semibold tracking-tight">
          <Logo />
          <span className="hidden sm:inline">{t('app.name')}</span>
        </Link>
        <BoardSwitcher />
        <div className="ml-auto flex items-center gap-0.5">
          <Button asChild variant="ghost" size="sm" className={cn(onAgents && 'bg-muted')}>
            <Link to="/agents" aria-current={onAgents ? 'page' : undefined}>
              <BotIcon />
              <span className="hidden md:inline">{t('nav.agents')}</span>
            </Link>
          </Button>
          <LanguageMenu />
          <ThemeMenu />
        </div>
      </header>
      <main id="main" tabIndex={-1} className="flex min-h-0 flex-1 flex-col overflow-auto outline-none">
        <Outlet />
      </main>
    </div>
  )
}
