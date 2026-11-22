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
    <SearchBox query={query} onChange={setQuery} noResults={false} truncated={false} syntaxError="" searching={false} active={false} />
  )
}

describe('SearchBox 清除按钮', () => {
  it('输入后用清空按钮替代搜索帮助，清空后恢复帮助与输入焦点', async () => {
    renderApp(<Harness />)
    const user = userEvent.setup()
    const helpName = { name: en.search.syntax }
    const clearName = { name: en.search.clear }
    expect(screen.getByRole('button', helpName)).toBeVisible()
    expect(screen.queryByRole('button', clearName)).not.toBeInTheDocument()
    const input = screen.getByPlaceholderText(en.search.placeholder)
    await user.type(input, 'tag:bug')
    expect(screen.queryByRole('button', helpName)).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', clearName))
    expect(input).toHaveValue('')
    expect(input).toHaveFocus()
    expect(screen.getByRole('button', helpName)).toBeVisible()
    expect(screen.queryByRole('button', clearName)).not.toBeInTheDocument()
  })

  it('空输入时可以打开帮助，点选示例后填入并聚焦搜索框', async () => {
    renderApp(<Harness />)
    const user = userEvent.setup()
    const input = screen.getByPlaceholderText(en.search.placeholder)
    await user.click(screen.getByRole('button', { name: en.search.syntax }))
    await user.click(screen.getByRole('button', { name: en.search.examples.openBugs }))
    expect(input).toHaveValue(en.search.examples.openBugs)
    expect(input).toHaveFocus()
    expect(screen.getByRole('button', { name: en.search.clear })).toBeVisible()
  })
})

it('搜索有匹配时不显示过滤比例', () => {
  renderApp(<SearchBox query="tag:bug" onChange={() => {}} noResults={false} truncated={false} syntaxError="" searching={false} active />)
  expect(screen.queryByText(/\d+\s*\/\s*\d+/)).not.toBeInTheDocument()
})
