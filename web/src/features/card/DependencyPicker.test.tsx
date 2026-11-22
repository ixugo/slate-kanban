import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { CardRef } from '@/lib/api/types'
import { renderApp } from '@/test/renderApp'
import { DependencyPicker } from './DependencyPicker'

function dependency(number: number, title: string): CardRef {
  return { id: 200 + number, key: `TST-${number}`, title, column: 'To do', done: false }
}

describe('DependencyPicker', () => {
  it('limits the unfiltered list to the 20 newest numbers, but searches all candidates', async () => {
    const options = Array.from({ length: 27 }, (_, index) => dependency(index + 1, `Task ${index + 1}`))
    const onChange = vi.fn()
    renderApp(<DependencyPicker cardId={227} selected={[225]} options={options} onChange={onChange} />)
    const list = () => within(screen.getByRole('list', { name: 'Add dependency' }))
    const initial = list().getAllByRole('button').map((option) => option.getAttribute('aria-label'))
    expect(initial).toHaveLength(20)
    expect(initial.slice(0, 3)).toEqual(['#26 Task 26', '#24 Task 24', '#23 Task 23'])
    expect(initial.at(-1)).toBe('#6 Task 6')
    const user = userEvent.setup()
    const search = screen.getByRole('textbox', { name: 'Search by #number or title' })
    await user.type(search, '#3')
    expect(list().getAllByRole('button')).toHaveLength(1)
    await user.click(list().getByRole('button', { name: '#3 Task 3' }))
    expect(onChange).toHaveBeenCalledWith([225, 203])
    await user.type(search, '  tAsK 2  ')
    expect(list().getAllByRole('button')[0]).toHaveAccessibleName('#26 Task 26')
    expect(list().getByRole('button', { name: '#2 Task 2' })).toBeInTheDocument()
  })

  it('truncates the visible title by Unicode characters while matching its whole text', async () => {
    const title = '多字节😀标题可以截断文本'
    renderApp(<DependencyPicker cardId={201} selected={[]} options={[dependency(2, title)]} onChange={() => {}} />)
    const button = screen.getByRole('button', { name: `#2 ${title}` })
    expect(within(button).getByTitle(title)).toHaveTextContent('多字节😀标题可以截断…')
    await userEvent.type(screen.getByRole('textbox'), '文本')
    expect(screen.getByRole('button', { name: `#2 ${title}` })).toBeInTheDocument()
  })
})
