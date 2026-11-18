// 顶栏上的看板切换：显示当前看板的名字，点开可以跳到别的看板或回到看板列表。

import { CheckIcon, ChevronsUpDownIcon } from 'lucide-react'
import { useTranslation } from '@/i18n/react'
import { useMatch, useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useBoards } from '@/features/boards/useBoards'
import { parseId } from '@/lib/route'

/** 看板切换菜单；不在某块看板页面时按钮显示"看板" */
export function BoardSwitcher() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const match = useMatch('/b/:boardId/*')
  const currentId = parseId(match?.params.boardId)
  const { data: boards } = useBoards()
  const current = boards?.find((b) => b.id === currentId)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="sm" className="max-w-40 sm:max-w-56">
          <span className="truncate">{current?.name ?? t('nav.boards')}</span>
          <ChevronsUpDownIcon className="opacity-60" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        <DropdownMenuLabel>{t('nav.switchBoard')}</DropdownMenuLabel>
        {boards?.map((board) => (
          <DropdownMenuItem key={board.id} onSelect={() => navigate(`/b/${board.id}`)}>
            <span className="font-mono text-[10px] text-muted-foreground">{board.slug}</span>
            <span className="truncate">{board.name}</span>
            {board.id === currentId && <CheckIcon className="ml-auto" />}
          </DropdownMenuItem>
        ))}
        {boards && boards.length > 0 && <DropdownMenuSeparator />}
        <DropdownMenuItem onSelect={() => navigate('/')}>{t('nav.allBoards')}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
