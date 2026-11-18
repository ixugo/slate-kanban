// 看板切换：原生 select，第一项回列表；列表未加载时只显示占位文字，不撑破顶栏。

import { ChevronDownIcon } from 'lucide-react'
import type { ChangeEvent } from 'react'
import { cn } from 'cn'
import { useTranslation } from '@/i18n/react'
import { useMatch, useNavigate } from 'react-router-dom'
import { useBoards } from '@/features/boards/useBoards'
import { parseId } from '@/lib/route'

const HOME_VALUE = '__home__'

/** 顶栏看板选择框；不在看板页且列表已加载时选中「所有看板」 */
export function BoardSwitcher() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const match = useMatch('/b/:boardId/*')
  const currentId = parseId(match?.params.boardId)
  const { data: boards } = useBoards()
  const current = boards?.find((b) => b.id === currentId)

  const onChange = (event: ChangeEvent<HTMLSelectElement>) => {
    const value = event.target.value
    if (value === HOME_VALUE) navigate('/')
    else navigate(`/b/${value}`)
  }

  if (!boards) {
    return (
      <span
        className="inline-flex h-7 max-w-28 items-center truncate px-2 text-xs text-muted-foreground sm:max-w-40"
        aria-busy="true"
      >
        {current?.name ?? t('nav.boards')}
      </span>
    )
  }

  const selectValue = currentId !== undefined ? String(currentId) : HOME_VALUE

  return (
    <div
      className={cn(
        'relative inline-flex h-7 max-w-32 items-center sm:max-w-44',
        'rounded-none border border-transparent focus-within:border-ring focus-within:ring-1 focus-within:ring-ring/50',
      )}
    >
      <select
        aria-label={t('nav.switchBoard')}
        value={selectValue}
        onChange={onChange}
        className={cn(
          'h-7 w-full min-w-0 cursor-pointer appearance-none truncate rounded-none border-0 bg-transparent py-0 pl-2 pr-6 text-xs outline-none',
          'hover:bg-muted dark:hover:bg-muted/50',
        )}
      >
        <option value={HOME_VALUE}>{t('nav.allBoards')}</option>
        {boards.map((board) => (
          <option key={board.id} value={String(board.id)}>
            {`${board.slug} ${board.name}`}
          </option>
        ))}
      </select>
      <ChevronDownIcon
        className="pointer-events-none absolute right-1 size-3.5 shrink-0 text-muted-foreground"
        aria-hidden
      />
    </div>
  )
}