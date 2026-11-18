import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import '@/i18n'
import { useDocumentTitle } from './useDocumentTitle'

// 只调用 hook 的小组件
function Page({ title }: { title?: string }) {
  useDocumentTitle(title)
  return null
}

describe('useDocumentTitle', () => {
  it('标题是"页面名 · 应用名"', () => {
    render(<Page title="Boards" />)
    expect(document.title).toBe('Boards · Slate')
  })

  it('没有页面名时只显示应用名', () => {
    render(<Page />)
    expect(document.title).toBe('Slate')
  })

  it('页面名变化时跟着变，离开页面恢复成应用名', () => {
    const { rerender, unmount } = render(<Page title="A" />)
    rerender(<Page title="B" />)
    expect(document.title).toBe('B · Slate')
    unmount()
    expect(document.title).toBe('Slate')
  })
})
