import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { en } from '@/i18n/locales/en'
import { renderApp } from '@/test/renderApp'
import { SearchBox } from './SearchBox'

/** 受控外壳：和看板页一样，文字存在父组件里 */
function Harness() {
  const [query, setQuery] = useState('')
  return (
    <SearchBox query={query} onChange={setQuery} matched={0} total={0} truncated={false} syntaxError="" searching={false} active={false} />
  )
}

describe('SearchBox 清除按钮', () => {
  it('有文字才出现；点击清空文字并把焦点放回输入框', async () => {
    renderApp(<Harness />)
    expect(screen.queryByRole('button', { name: en.search.clear })).not.toBeInTheDocument()
    const input = screen.getByPlaceholderText(en.search.placeholder)
    await userEvent.type(input, 'tag:bug')
    await userEvent.click(screen.getByRole('button', { name: en.search.clear }))
    expect(input).toHaveValue('')
    expect(input).toHaveFocus()
    expect(screen.queryByRole('button', { name: en.search.clear })).not.toBeInTheDocument()
  })
})
