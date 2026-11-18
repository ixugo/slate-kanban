// 所有页面共用的外框：顶栏（标志、看板切换、agent、语言、主题、退出）加页面内容区。

import { BotIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { Link, Outlet, useMatch } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { LogoutButton } from '@/features/auth/LogoutButton'
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
        className="sr-only z-50 rounded-full bg-background px-4 py-2 text-ui shadow-float focus:not-sr-only focus:absolute focus:top-3 focus:left-3"
      >
        {t('nav.skipToContent')}
      </a>
      <header className="material-bar z-30 flex h-14 shrink-0 items-center gap-1.5 px-3 shadow-[0_0.5px_0_var(--border)] sm:px-5">
        <Link
          to="/"
          className="mr-1 flex items-center gap-2.5 rounded-full py-1 pr-3 pl-1 text-body font-semibold tracking-tight transition-colors duration-200 outline-none hover:bg-secondary focus-visible:ring-[3px] focus-visible:ring-ring/40"
        >
          <Logo className="size-7" />
          <span className="hidden sm:inline">{t('app.name')}</span>
        </Link>
        <BoardSwitcher />
        <div className="ml-auto flex items-center gap-0.5">
          <Button asChild variant="ghost" className={cn(onAgents && 'bg-secondary text-foreground')}>
            <Link to="/agents" aria-current={onAgents ? 'page' : undefined}>
              <BotIcon />
              <span className="hidden md:inline">{t('nav.agents')}</span>
            </Link>
          </Button>
          <LanguageMenu />
          <ThemeMenu />
          <LogoutButton />
        </div>
      </header>
      <main id="main" tabIndex={-1} className="flex min-h-0 flex-1 flex-col overflow-auto outline-none">
        <Outlet />
      </main>
    </div>
  )
}
