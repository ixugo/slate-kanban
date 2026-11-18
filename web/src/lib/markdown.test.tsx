import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Markdown } from './markdown'

describe('Markdown', () => {
  it('渲染标题与粗体，原始 HTML 当作文本', () => {
    render(<Markdown source={'# 标题\n\n**粗体** 与 <script>alert(1)</script>'} />)
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('标题')
    expect(screen.getByText('粗体')).toBeInTheDocument()
    expect(document.querySelector('script')).toBeNull()
    expect(screen.getByText(/<script>/)).toBeInTheDocument()
  })

  it('拒绝 javascript: 链接，安全链接带 rel', () => {
    render(<Markdown source={'[坏](javascript:alert(1)) [好](https://example.com)'} />)
    expect(screen.queryByRole('link', { name: '坏' })).toBeNull()
    const good = screen.getByRole('link', { name: '好' })
    expect(good).toHaveAttribute('href', 'https://example.com')
    expect(good).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('不渲染图片标签语法为 HTML 图片', () => {
    render(<Markdown source={'![x](https://evil.com/x.png)'} />)
    expect(screen.queryByRole('img')).toBeNull()
  })
})