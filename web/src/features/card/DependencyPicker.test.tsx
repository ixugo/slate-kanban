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
  it('returns the same exact dependency number with or without #', async () => {
    const options = [dependency(138, 'Target task'), dependency(139, 'Title mentions 138')]
    renderApp(<DependencyPicker cardId={201} selected={[]} options={options} onChange={() => {}} />)
    const user = userEvent.setup()
    const search = screen.getByRole('textbox')
    for (const query of ['#138', '138']) {
      await user.clear(search)
      await user.type(search, query)
      const list = within(screen.getByRole('list', { name: 'Add dependency' }))
      expect(list.getAllByRole('listitem')).toHaveLength(1)
      expect(list.getByText('#138')).toBeInTheDocument()
    }
  })

  it('limits the unfiltered list to the 20 newest numbers, but searches all candidates', async () => {
    const options = Array.from({ length: 27 }, (_, index) => dependency(index + 1, `Task ${index + 1}`))
    const onChange = vi.fn()
    renderApp(<DependencyPicker cardId={227} selected={[225]} options={options} onChange={onChange} />)
    const list = () => within(screen.getByRole('list', { name: 'Add dependency' }))
    const initial = list().getAllByRole('checkbox').map((option) => option.getAttribute('aria-label'))
    expect(initial).toHaveLength(20)
    expect(initial.slice(0, 3)).toEqual(['#26 Task 26', '#25 Task 25', '#24 Task 24'])
    expect(initial.at(-1)).toBe('#7 Task 7')
    expect(list().getByRole('checkbox', { name: '#25 Task 25' })).toBeChecked()
    const user = userEvent.setup()
    const search = screen.getByRole('textbox', { name: 'Search by #number or title' })
    await user.type(search, '#3')
    expect(list().getAllByRole('checkbox')).toHaveLength(1)
    await user.click(list().getByRole('checkbox', { name: '#3 Task 3' }))
    expect(onChange).toHaveBeenCalledWith([225, 203])
    await user.clear(search)
    await user.type(search, '  tAsK 2  ')
    expect(list().getAllByRole('checkbox')[0]).toHaveAccessibleName('#26 Task 26')
    expect(list().getByRole('checkbox', { name: '#2 Task 2' })).toBeInTheDocument()
  })

  it('truncates the visible title by Unicode characters while matching its whole text', async () => {
    const title = '多字节😀标题可以截断文本'
    renderApp(<DependencyPicker cardId={201} selected={[]} options={[dependency(2, title)]} onChange={() => {}} />)
    const button = screen.getByRole('checkbox', { name: `#2 ${title}` })
    expect(within(button).getByTitle(title)).toHaveTextContent('多字节😀标题可以截断…')
    await userEvent.type(screen.getByRole('textbox'), '文本')
    expect(screen.getByRole('checkbox', { name: `#2 ${title}` })).toBeInTheDocument()
  })
})
