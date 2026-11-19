import { describe, expect, it } from 'vitest'
import { renderLines } from './render'

/** 把逐行 HTML 放进真实 DOM，方便按元素检查 */
function mount(value: string, sel: { lo: number; hi: number } | null = null): HTMLElement[] {
  const box = document.createElement('div')
  box.innerHTML = renderLines(value, sel).join('')
  return Array.from(box.children) as HTMLElement[]
}

/** 当前看得见的文字：去掉带 md-hide / md-off 的元素 */
function visible(el: HTMLElement): string {
  const clone = el.cloneNode(true) as HTMLElement
  clone.querySelectorAll('.md-hide').forEach((n) => n.remove())
  return el.classList.contains('md-off') ? '' : (clone.textContent ?? '')
}

describe('renderLines 不丢字', () => {
  const docs = [
    '',
    'a',
    '# 标题 **粗** 和 *斜*\n\n正文 ~~删~~ <u>线</u> `代码` [链](https://a.com) https://b.com\\*\n> 引用 **x**\n---\n```js\n<b>&"x"</b>\n```',
    '- a\n  续行 **b\n  c**\n  - [ ] 子\n    - [x] 孙\n1. 一\n2) 二\n\n未闭合 **粗\n后一行**',
    '```\n没闭合\n- 不是列表',
    '&lt;script&gt;<img src=x onerror=alert(1)>',
  ]
  it.each(docs.map((d, i) => [i, d] as const))('文档 %i：每行文字等于原文，行数一致', (_i, doc) => {
    const lines = doc.split('\n')
    for (const sel of [null, { lo: 0, hi: 0 }, { lo: 0, hi: doc.length }, { lo: doc.length, hi: doc.length }]) {
      const els = mount(doc, sel)
      expect(els.map((e) => e.textContent)).toEqual(lines)
    }
  })

  it('图片附件：不丢字；光标没碰到时只显示图片，碰到时记号和图片都在', () => {
    const doc = '前 ![截图](/api/v1/attachments/0123456789abcdef0123456789abcdef) 后'
    const off = mount(doc)[0]
    expect(off.textContent).toBe(doc)
    expect(off.querySelector('img.md-img')).toHaveAttribute('src', '/api/v1/attachments/0123456789abcdef0123456789abcdef')
    expect(visible(off)).toBe('前  后')
    const on = mount(doc, { lo: 3, hi: 3 })[0]
    expect(visible(on)).toBe(doc)
    expect(on.querySelector('img.md-img')).not.toBeNull()
  })

  it('只有一张图片的行也不补 <br>，外部图片地址仍是文字', () => {
    expect(mount('![a](/api/v1/attachments/0123456789abcdef0123456789abcdef)')[0].querySelector('br')).toBeNull()
    const ext = mount('![a](https://e.com/x.png)')[0]
    expect(ext.querySelector('img')).toBeNull()
    expect(ext.textContent).toBe('![a](https://e.com/x.png)')
  })

  it('用户文字里的标签不会变成元素', () => {
    const els = mount('<img src=x onerror=alert(1)>')
    expect(els[0].querySelector('img')).toBeNull()
    expect(els[0].textContent).toBe('<img src=x onerror=alert(1)>')
  })

  it('随机文档：不抛错、不丢字', () => {
    const atoms = ['**', '*', '_', '~~', '<u>', '</u>', '`', '[a](https://x.y)', '# ', '> ', '- ', '- [ ] ', '1. ', '---', '```', ' ', '  ', '\n', '\n', 'x', '中', '\\', '<', '&']
    let seed = 7
    const next = (n: number) => ((seed = (seed * 1103515245 + 12345) >>> 0) % n)
    for (let k = 0; k < 300; k++) {
      let doc = ''
      for (let i = next(30); i > 0; i--) doc += atoms[next(atoms.length)]
      const lo = next(doc.length + 1)
      const hi = lo + next(doc.length - lo + 1)
      const els = mount(doc, k % 3 === 0 ? null : { lo, hi })
      expect(els.map((e) => e.textContent).join('\n')).toBe(doc)
    }
  })
})

describe('块级外观', () => {
  it('标题：没碰到时记号藏起来，碰到时亮出来', () => {
    const [off] = mount('## 二级', null)
    expect(off.className).toContain('md-h2')
    expect(visible(off)).toBe('二级')
    const [on] = mount('## 二级', { lo: 3, hi: 3 })
    expect(visible(on)).toBe('## 二级')
  })

  it('引用带层数', () => {
    const [q] = mount('>> 两层')
    expect(q.dataset.q).toBe('2')
    expect(visible(q)).toBe('两层')
  })

  it('分隔线：没碰到时不显示文字', () => {
    expect(visible(mount('---')[0])).toBe('')
    expect(visible(mount('---', { lo: 1, hi: 1 })[0])).toBe('---')
  })

  it('空行是 md-blank，且带 <br> 撑出高度', () => {
    const [a] = mount('a\n\nb')
    expect(mount('a\n\nb')[1].className).toContain('md-blank')
    expect(mount('a\n\nb')[1].innerHTML).toBe('<br>')
    expect(a.className).toContain('md-p')
  })

  it('光标所在的空行不用窄缝：空编辑区和新按出的空行，光标和开始输入后一样高', () => {
    expect(mount('', { lo: 0, hi: 0 })[0].className).not.toContain('md-blank')
    expect(mount('', null)[0].className).toContain('md-blank')
    // 光标在第 3 行（下标 2）：中间的空行还是窄缝，光标所在的空行是正常行高
    expect(mount('a\n\n\nb', { lo: 3, hi: 3 }).map((el) => el.className.includes('md-blank'))).toEqual([false, true, false, false])
  })

  it('围栏代码：有代码行时围栏行平时藏起来，光标进入代码块后亮出来', () => {
    const doc = '```js\nlet a\n```'
    const off = mount(doc)
    expect(off.map((e) => e.className.includes('md-off'))).toEqual([true, false, true])
    expect(off[1].className).toContain('md-cf')
    expect(off[1].className).toContain('md-cl')
    const on = mount(doc, { lo: 7, hi: 7 })
    expect(on.map((e) => e.className.includes('md-off'))).toEqual([false, false, false])
  })

  it('空的围栏代码块两行都保持可见', () => {
    expect(mount('```\n```').map((e) => e.className.includes('md-off'))).toEqual([false, false])
  })

  it('代码块里的内容不按 Markdown 解析', () => {
    const [, code] = mount('```\n**x**\n```')
    expect(code.querySelector('.md-b')).toBeNull()
  })
})

describe('列表外观', () => {
  it('缩进层数、圆点和编号', () => {
    const els = mount('- a\n  - b\n    - c\n- d\n3. x\n4. y')
    expect(els.map((e) => e.dataset.d)).toEqual(['0', '1', '2', '0', '0', '0'])
    expect(els.map((e) => e.dataset.b)).toEqual(['•', '◦', '▪', '•', '3.', '4.'])
  })

  it('行首的缩进和记号一直藏着，只露出正文', () => {
    const [li] = mount('  - [x] 好', { lo: 8, hi: 8 })
    expect(li.dataset.task).toBe('1')
    expect(visible(li)).toBe('好')
  })

  it('续行跟着所属项的层级，缩进也藏起来', () => {
    const cont = mount('- a\n  - b\n    接着')[2]
    expect(cont.dataset.d).toBe('1')
    expect(visible(cont)).toBe('接着')
  })

  it('空列表项也有 <br>，光标放得进去', () => {
    expect(mount('- ')[0].innerHTML).toContain('<br>')
  })
})

describe('行内格式', () => {
  it('粗体：光标在里面或贴着边时记号亮出来，离开就藏起来', () => {
    const doc = 'a **粗** b'
    expect(visible(mount(doc, { lo: 0, hi: 0 })[0])).toBe('a 粗 b')
    expect(visible(mount(doc, { lo: 1, hi: 1 })[0])).toBe('a 粗 b')
    for (const at of [2, 4, 7]) expect(visible(mount(doc, { lo: at, hi: at })[0])).toBe('a **粗** b')
    expect(visible(mount(doc, { lo: 8, hi: 8 })[0])).toBe('a 粗 b')
    expect(mount(doc)[0].querySelector('.md-b')?.textContent).toBe('粗')
  })

  it('没有焦点（选区为 null）时所有记号都藏起来', () => {
    expect(visible(mount('**a** *b* ~~c~~ `d`', null)[0])).toBe('a b c d')
  })

  it('嵌套格式叠加样式', () => {
    const el = mount('***粗斜***')[0]
    const inner = el.querySelector('.md-b.md-i, .md-i.md-b')
    expect(inner?.textContent).toBe('粗斜')
  })

  it('链接：文字带 data-href，地址记号藏起来', () => {
    const el = mount('[官网](https://a.com)')[0]
    const a = el.querySelector('.md-a') as HTMLElement
    expect(a.textContent).toBe('官网')
    expect(a.dataset.href).toBe('https://a.com')
    expect(visible(el)).toBe('官网')
  })

  it('裸网址也是链接，没有记号', () => {
    const a = mount('见 https://a.com/x')[0].querySelector('.md-a') as HTMLElement
    expect(a.dataset.href).toBe('https://a.com/x')
  })

  it('行内代码、下划线、删除线', () => {
    const el = mount('`c` <u>u</u> ~~s~~')[0]
    expect(el.querySelector('.md-ic')?.textContent).toBe('c')
    expect(el.querySelector('.md-u')?.textContent).toBe('u')
    expect(el.querySelector('.md-s')?.textContent).toBe('s')
  })

  it('反斜杠转义：反斜杠藏起来，字符留着', () => {
    expect(visible(mount('\\*a')[0])).toBe('*a')
  })

  it('跨行的格式：两行都带样式，选区碰到任意一行两处记号都亮', () => {
    const doc = '**a\nb**'
    const off = mount(doc)
    expect(off[0].querySelector('.md-b')?.textContent).toBe('a')
    expect(off[1].querySelector('.md-b')?.textContent).toBe('b')
    const on = mount(doc, { lo: 5, hi: 5 })
    expect(visible(on[0])).toBe('**a')
    expect(visible(on[1])).toBe('b**')
  })

  it('标题和引用里的行内格式也生效', () => {
    expect(mount('# **x**')[0].querySelector('.md-b')?.textContent).toBe('x')
    expect(mount('> *y*')[0].querySelector('.md-i')?.textContent).toBe('y')
  })
})
