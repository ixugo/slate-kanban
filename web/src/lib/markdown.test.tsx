import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { toggleTaskAt } from '@/lib/md/blocks'
import { Markdown } from './markdown'

/** 渲染后取出容器，方便按标签名查结构 */
function renderMd(source: string, props: { onToggleTask?: (line: number) => void } = {}) {
  return render(<Markdown source={source} {...props} />).container.firstElementChild as HTMLElement
}

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

  it('外部图片不加载，本服务的图片附件显示成可点开的图片', () => {
    render(<Markdown source={'![x](https://evil.com/x.png)'} />)
    expect(screen.queryByRole('img')).toBeNull()
    cleanup()
    render(<Markdown source={'![截图](/api/v1/attachments/0123456789abcdef0123456789abcdef)'} />)
    const img = screen.getByRole('img', { name: '截图' })
    expect(img).toHaveAttribute('src', '/api/v1/attachments/0123456789abcdef0123456789abcdef')
    expect(img.closest('a')).toHaveAttribute('target', '_blank')
  })
})

describe('Markdown：行内格式', () => {
  it('删除线、下划线、斜体、行内代码各用对应标签', () => {
    const root = renderMd('~~删除~~ <u>下划线</u> *斜体* `代码`')
    expect(root.querySelector('del')).toHaveTextContent('删除')
    expect(root.querySelector('u')).toHaveTextContent('下划线')
    expect(root.querySelector('em')).toHaveTextContent('斜体')
    expect(root.querySelector('code')).toHaveTextContent('代码')
  })

  it('中文标点紧贴的粗体也能渲染，转义的星号当文字', () => {
    const root = renderMd('**注意：**请看 \\*不是斜体\\*')
    expect(root.querySelector('strong')).toHaveTextContent('注意：')
    expect(root.querySelector('em')).toBeNull()
    expect(root).toHaveTextContent('*不是斜体*')
  })

  it('裸网址自动变成链接，句末的中文标点不算进地址', () => {
    renderMd('见 https://example.com/a。')
    const link = screen.getByRole('link', { name: 'https://example.com/a' })
    expect(link).toHaveAttribute('href', 'https://example.com/a')
    expect(link).toHaveAttribute('target', '_blank')
  })
})

describe('Markdown：块级结构', () => {
  it('六级标题都渲染成对应级别', () => {
    renderMd('# a\n\n## b\n\n### c\n\n#### d\n\n##### e\n\n###### f')
    for (let level = 1; level <= 6; level++) {
      expect(screen.getByRole('heading', { level })).toBeInTheDocument()
    }
  })

  it('嵌套列表渲染成嵌套的 ul，有序列表保留起始数字', () => {
    const root = renderMd('- a\n  - b\n- c\n\n3. x\n4. y')
    const outer = root.querySelector(':scope > ul') as HTMLElement
    expect(outer.querySelectorAll(':scope > li')).toHaveLength(2)
    expect(within(outer.querySelector('li') as HTMLElement).getByText('b').closest('ul')).not.toBe(outer)
    expect(root.querySelector('ol')).toHaveAttribute('start', '3')
  })

  it('引用可以嵌套，代码块和分隔线按原样输出', () => {
    const root = renderMd('> a\n> > b\n\n---\n\n```go\nx := **1**\n```')
    expect(root.querySelectorAll('blockquote')).toHaveLength(2)
    expect(root.querySelector('hr')).not.toBeNull()
    expect(root.querySelector('pre code')).toHaveTextContent('x := **1**')
    expect(root.querySelector('pre strong')).toBeNull()
  })

  it('处理表格列对齐、转义管道和长度不齐的正文', () => {
    const root = renderMd([
      '左 | 中 | 右',
      ':--- | :---: | ---:',
      '*甲\\|乙* | `a\\|b` | [链接](https://example.com)',
      '短行 | 空末列',
      '| 甲 | 乙 | 丙 | 多余 |',
      '> 表格外的引用',
    ].join('\n'))
    const table = within(screen.getByRole('table'))
    expect(table.getAllByRole('columnheader').map((cell) => cell.style.textAlign)).toEqual(['left', 'center', 'right'])
    expect(root.querySelector('td em')).toHaveTextContent('甲|乙')
    expect(root.querySelector('td code')).toHaveTextContent('a|b')
    expect(table.getByRole('link', { name: '链接' })).toHaveAttribute('href', 'https://example.com')
    expect(table.getAllByRole('cell')).toHaveLength(9)
    expect(table.getAllByRole('cell')[5]).toBeEmptyDOMElement()
    expect(table.queryByText('多余')).not.toBeInTheDocument()
    expect(root.querySelector('blockquote')).toHaveTextContent('表格外的引用')
  })

  it('列定义不匹配或表格在代码围栏中时保留原文', () => {
    const root = renderMd('| A | B |\n| --- |\n\n```md\n| A | B |\n| --- | --- |\n```')
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(root.querySelector('p')).toHaveTextContent('| A | B |')
    expect(root.querySelector('pre code')).toHaveTextContent('| --- | --- |')
  })
})

describe('Markdown：任务清单', () => {
  it('没有回调时勾选框只读，状态和原文一致', () => {
    renderMd('- [ ] 买菜\n- [x] 做饭')
    const boxes = screen.getAllByRole('checkbox')
    expect(boxes).toHaveLength(2)
    expect(boxes.every((b) => (b as HTMLInputElement).disabled)).toBe(true)
    expect(screen.getByRole('checkbox', { name: '买菜' })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: '做饭' })).toBeChecked()
  })

  it('有回调时点勾选框会带上任务所在的行号，嵌套任务也对', async () => {
    const onToggle = vi.fn()
    renderMd('# 清单\n\n- [ ] 外层\n  - [ ] 里层', { onToggleTask: onToggle })
    await userEvent.click(screen.getByRole('checkbox', { name: '里层' }))
    expect(onToggle).toHaveBeenCalledWith(3)
    await userEvent.click(screen.getByRole('checkbox', { name: /外层/ }))
    expect(onToggle).toHaveBeenLastCalledWith(2)
  })

  it('配合 toggleTaskAt 改写原文后，勾选状态随之更新', async () => {
    function Harness() {
      const [source, setSource] = useState('- [ ] a\n- [x] b')
      return <Markdown source={source} onToggleTask={(line) => setSource(toggleTaskAt(source, line))} />
    }
    render(<Harness />)
    await userEvent.click(screen.getByRole('checkbox', { name: 'a' }))
    expect(screen.getByRole('checkbox', { name: 'a' })).toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: 'b' }))
    expect(screen.getByRole('checkbox', { name: 'b' })).not.toBeChecked()
  })

  it('同一页面里有多处 Markdown 时，勾选框的名称不会串', () => {
    render(
      <>
        <Markdown source="- [ ] 甲" />
        <Markdown source="- [x] 乙" />
      </>,
    )
    expect(screen.getByRole('checkbox', { name: '甲' })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: '乙' })).toBeChecked()
  })
})
