import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Button } from './button'

describe('Button 样式类', () => {
  it.each(['default', 'outline', 'secondary', 'ghost', 'destructive'] as const)('大尺寸的 %s 变体保留文字颜色', (variant) => {
    render(<Button size="lg" variant={variant}>确定</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toMatch(/(^|\s)text-(primary-foreground|foreground\/80|foreground|secondary-foreground|destructive)(\s|$)/)
  })

  it('大尺寸默认按钮同时有主色背景、对比文字色和正文字号', () => {
    render(<Button size="lg">新建项目</Button>)
    const cls = screen.getByRole('button').className
    expect(cls).toContain('bg-primary')
    expect(cls).toContain('text-primary-foreground')
    expect(cls).toContain('text-[length:var(--text-body)]')
  })
})
