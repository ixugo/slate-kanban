import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderApp } from '@/test/renderApp'
import { makeCard } from '@/test/factories'
import { UnreadBadge } from './UnreadBadge'
import { CardTile } from './CardTile'

describe('UnreadBadge', () => {
  it('带无障碍标签', () => {
    renderApp(<UnreadBadge count={3} />)
    expect(screen.getByLabelText('3 unread updates')).toBeInTheDocument()
  })
  it('0 不显示', () => {
    renderApp(<UnreadBadge count={0} />)
    expect(screen.queryByLabelText(/unread/)).not.toBeInTheDocument()
  })
  it('超过 99 显示 99+', () => {
    renderApp(<UnreadBadge count={100} />)
    expect(screen.getByText('99+')).toBeInTheDocument()
  })
})

describe('CardTile unread', () => {
  it('有未读时显示角标', () => {
    renderApp(
      <CardTile card={makeCard({ id: 1, unread_count: 5 })} />,
      { route: '/b/1' },
    )
    expect(screen.getByText('5')).toBeInTheDocument()
  })
})