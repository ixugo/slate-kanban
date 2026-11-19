import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import { LiveStatus } from './LiveStatus'

describe('LiveStatus', () => {
  it('已连接时什么都不显示', () => {
    const { container } = render(<LiveStatus state="live" onRetry={() => {}} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('连接中显示提示文字', () => {
    render(<LiveStatus state="connecting" onRetry={() => {}} />)
    expect(screen.getByText(en.stream.connecting)).toBeInTheDocument()
  })

  it('断线时显示提示并能点重试', async () => {
    const onRetry = vi.fn()
    render(<LiveStatus state="offline" onRetry={onRetry} />)
    expect(screen.getByText(en.stream.offline)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: en.common.retry }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('项目已被删除时提示不存在，即使连接状态是 live', () => {
    render(<LiveStatus state="live" gone onRetry={() => {}} />)
    expect(screen.getByText(en.stream.notFound)).toBeInTheDocument()
  })
})
