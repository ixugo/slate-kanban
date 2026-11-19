import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import '@/i18n'
import { en } from '@/i18n/locales/en'
import { ErrorBoundary } from './ErrorBoundary'

// 渲染时一定抛错的组件
function Boom(): never {
  throw new Error('boom detail')
}

afterEach(() => vi.restoreAllMocks())

describe('ErrorBoundary', () => {
  it('子组件正常时原样渲染', () => {
    render(
      <ErrorBoundary>
        <p>hello</p>
      </ErrorBoundary>,
    )
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('子组件渲染出错：显示兜底界面（标题、错误信息、重新加载），并把错误记到控制台', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    render(
      <ErrorBoundary>
        <Boom />
      </ErrorBoundary>,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(en.errors.title)
    expect(screen.getByRole('alert')).toHaveTextContent('boom detail')
    expect(screen.getByRole('button', { name: en.common.reload })).toBeInTheDocument()
    expect(spy.mock.calls.some((c) => c[0] === 'slate: render error')).toBe(true)
  })
})
