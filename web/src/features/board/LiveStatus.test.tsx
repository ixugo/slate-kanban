import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { LiveStatus } from './LiveStatus'

describe('LiveStatus', () => {
  it('已连接时圆点为绿色', () => {
    const { container } = render(<LiveStatus state="live" onRetry={() => {}} />)
    const dot = container.querySelector('.rounded-full')
    expect(dot?.className).toContain('bg-emerald-500')
  })
})