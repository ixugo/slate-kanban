import { describe, expect, it } from 'vitest'
import { inlineSource, isSafeHref, parseInline, type Inline } from './inline'

/** 把节点树压成一行好读的字符串，方便在用例里比较结构 */
function show(nodes: readonly Inline[]): string {
  return nodes
    .map((n) => {
      switch (n.kind) {
        case 'text':
          // 文字里的 < 转成 &lt;，这样“被认成下划线标签”和“当成普通文字”在比较时不会混为一谈
          return n.text.replaceAll('<', '&lt;')
        case 'esc':
          return `{esc:${n.char}}`
        case 'code':
          return `{code:${n.text}}`
        case 'span':
          return `<${n.tag}>${show(n.children)}</${n.tag}>`
        case 'link':
          return `{a:${n.href}|${show(n.children)}}`
      }
    })
    .join('')
}

const md = (src: string) => show(parseInline(src))

/** 沿树找出所有链接节点 */
function links(nodes: readonly Inline[]): Extract<Inline, { kind: 'link' }>[] {
  const out: Extract<Inline, { kind: 'link' }>[] = []
  for (const n of nodes) {
    if (n.kind === 'link') out.push(n, ...links(n.children))
    else if (n.kind === 'span') out.push(...links(n.children))
  }
  return out
}

describe('parseInline：基本记号', () => {
  it.each([
    ['纯文字', 'hello world', 'hello world'],
    ['空串', '', ''],
    ['粗体 **', '**bold**', '<strong>bold</strong>'],
    ['粗体 __', '__bold__', '<strong>bold</strong>'],
    ['斜体 *', '*it*', '<em>it</em>'],
    ['斜体 _', '_it_', '<em>it</em>'],
    ['删除线', '~~gone~~', '<del>gone</del>'],
    ['下划线标签', '<u>under</u>', '<u>under</u>'],
    ['行内代码', '`code`', '{code:code}'],
    ['双反引号的代码里可以有反引号', '``a`b``', '{code:a`b}'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('parseInline：嵌套与配对', () => {
  it.each([
    ['粗斜体', '***both***', '<em><strong>both</strong></em>'],
    ['粗体里嵌斜体', '**bold *it* more**', '<strong>bold <em>it</em> more</strong>'],
    ['斜体里嵌粗体', '*it **bold** it*', '<em>it <strong>bold</strong> it</em>'],
    ['粗体结尾紧跟斜体结尾', '**bold *it***', '<strong>bold <em>it</em></strong>'],
    ['两段并排', '*a* and **b**', '<em>a</em> and <strong>b</strong>'],
    ['删除线里嵌粗体', '~~**x**~~', '<del><strong>x</strong></del>'],
    ['下划线标签里嵌粗体', '<u>**x**</u>', '<u><strong>x</strong></u>'],
    ['粗体里有代码', '**a `b` c**', '<strong>a {code:b} c</strong>'],
    ['代码里的记号不生效', '`**x**`', '{code:**x**}'],
    ['__init__ 按粗体处理（和 GitHub 一致）', '__init__', '<strong>init</strong>'],
    ['三的倍数规则：*foo**bar**baz*', '*foo**bar**baz*', '<em>foo<strong>bar</strong>baz</em>'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('parseInline：不该变成强调的情况', () => {
  it.each([
    ['单词中间的下划线', 'snake_case_name', 'snake_case_name'],
    ['星号可以在单词中间', '2*3*4', '2<em>3</em>4'],
    ['前后有空格的星号', 'a * b * c', 'a * b * c'],
    ['空格包着的粗体标记', '** not bold **', '** not bold **'],
    ['没配对的标记原样保留', '**unclosed', '**unclosed'],
    ['孤零零的星号', '*', '*'],
    ['四个以上连续星号', '****', '****'],
    ['单个波浪线不是删除线', '~one~', '~one~'],
    ['大写标签不认', '<U>x</U>', '&lt;U>x&lt;/U>'],
    ['没有结束的下划线标签', '<u>x', '&lt;u>x'],
    ['空的下划线标签', '<u></u>', '&lt;u>&lt;/u>'],
    ['其它 HTML 当文字', '<script>alert(1)</script>', '&lt;script>alert(1)&lt;/script>'],
    ['没有结束的反引号', '`open', '`open'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('parseInline：转义', () => {
  it.each([
    ['转义星号', '\\*no\\*', '{esc:*}no{esc:*}'],
    ['转义反斜杠', 'a\\\\b', 'a{esc:\\}b'],
    ['反斜杠后不是标点就保留', '\\a', '\\a'],
    ['转义的星号不参与配对', '\\**a**', '{esc:*}<em>a</em>*'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('parseInline：链接', () => {
  it.each([
    ['链接', '[t](https://a.com)', '{a:https://a.com|t}'],
    ['链接文字里可以有粗体', '[**b**](https://a.com)', '{a:https://a.com|<strong>b</strong>}'],
    ['mailto 链接', '[m](mailto:a@b.c)', '{a:mailto:a@b.c|m}'],
    ['javascript 链接保持原文', '[x](javascript:alert(1))', '[x](javascript:alert(1))'],
    ['相对路径保持原文', '[x](/rel)', '[x](/rel)'],
    ['地址里带括号', '[a](https://x.com/a_(b))', '{a:https://x.com/a_(b)|a}'],
    ['地址里有空格不是链接，但裸网址照样成链接', '[a](https://x.com y)', '[a]({a:https://x.com|https://x.com} y)'],
    ['只有方括号', '[a] b', '[a] b'],
    ['方括号后面没有地址', '[a](', '[a]('],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })

  it('外部图片地址不产生图片，也不当成 [文字](地址) 链接', () => {
    const nodes = parseInline('![x](https://e.com/x.png)')
    expect(JSON.stringify(nodes)).not.toContain('image')
    expect(links(nodes).every((l) => l.open === '')).toBe(true)
    expect(show(nodes).startsWith('![x](')).toBe(true)
  })

  it.each([
    ['本服务附件', '![截图](/api/v1/attachments/0123456789abcdef0123456789abcdef)', true],
    ['夹在文字中间', '前 ![a](/api/v1/attachments/0123456789abcdef0123456789abcdef) 后', true],
    ['编号长度不对', '![a](/api/v1/attachments/abc)', false],
    ['编号带大写', '![a](/api/v1/attachments/0123456789ABCDEF0123456789ABCDEF)', false],
    ['路径多一段', '![a](/api/v1/attachments/0123456789abcdef0123456789abcdef/x)', false],
    ['别的路径', '![a](/api/v1/boards/1)', false],
    ['javascript 地址', '![a](javascript:alert(1))', false],
    ['说明里有方括号', '![a[b](/api/v1/attachments/0123456789abcdef0123456789abcdef)', false],
    ['地址里有空格', '![a](/api/v1/attachments/0123456789abcdef0123456789abcdef x)', false],
  ])('图片：%s', (_name, src, isImage) => {
    const nodes = parseInline(src)
    expect(JSON.stringify(nodes).includes('"kind":"image"')).toBe(isImage)
    expect(inlineSource(nodes)).toBe(src)
  })

  it('链接文字里不再嵌套链接', () => {
    const nodes = parseInline('[see https://in.com](https://out.com)')
    const found = links(nodes)
    expect(found.map((l) => l.href)).toEqual(['https://out.com'])
  })
})

describe('parseInline：裸网址自动成链接', () => {
  it.each([
    ['中间的网址', 'see https://a.com/x_y_z now', 'see {a:https://a.com/x_y_z|https://a.com/x_y_z} now'],
    ['网址后的标点不算网址', '(https://a.com).', '({a:https://a.com|https://a.com}).'],
    ['网址在粗体里', '**https://a.com**', '<strong>{a:https://a.com|https://a.com}</strong>'],
    ['单词中间的协议不算', 'xhttps://a.com', 'xhttps://a.com'],
    ['ftp 不自动链接', 'ftp://a.com', 'ftp://a.com'],
    ['只有协议没有主机', 'https://', 'https://'],
    ['紧跟的中文不算网址', '见https://a.com/x，请看', '见{a:https://a.com/x|https://a.com/x}，请看'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('parseInline：中文标点相邻时强调仍然生效', () => {
  it.each([
    ['中文粗体', '**中文**', '<strong>中文</strong>'],
    ['冒号结尾的粗体后面直接跟字', '**注意：**请看', '<strong>注意：</strong>请看'],
    ['引号开头的粗体前面直接是字', '请**“好”**吗', '请<strong>“好”</strong>吗'],
    ['中文斜体', '*强调*一下', '<em>强调</em>一下'],
  ])('%s', (_name, src, want) => {
    expect(md(src)).toBe(want)
  })
})

describe('isSafeHref', () => {
  it.each([
    ['https://a.com', true],
    ['HTTP://a.com', true],
    ['mailto:a@b.c', true],
    ['  https://a.com ', true],
    ['javascript:alert(1)', false],
    ['data:text/html,x', false],
    ['/relative', false],
    ['//evil.com', false],
    ['', false],
  ])('%j → %s', (href, want) => {
    expect(isSafeHref(href)).toBe(want)
  })
})

/** 带固定种子的伪随机数，让随机用例每次都一样、失败可复现 */
function lcg(seed: number): () => number {
  let s = seed
  return () => {
    s = (Math.imul(s, 1664525) + 1013904223) >>> 0
    return s / 2 ** 32
  }
}

describe('parseInline：随机输入下的不变量', () => {
  const alphabet = ['*', '_', '~', '`', '[', ']', '(', ')', '\\', '<', 'u', '>', '/', 'a', 'b', ' ', '\n', 'h', 't', 'p', 's', ':', '.', '!', '-', '#', '中', '：', '“']

  it('任何输入都能还原成原文，且所有链接地址都是安全协议', () => {
    const rand = lcg(20261118)
    for (let n = 0; n < 4000; n++) {
      const len = Math.floor(rand() * 60)
      let src = ''
      for (let i = 0; i < len; i++) src += alphabet[Math.floor(rand() * alphabet.length)]
      const nodes = parseInline(src)
      expect(inlineSource(nodes), JSON.stringify(src)).toBe(src)
      for (const link of links(nodes)) expect(isSafeHref(link.href), JSON.stringify(src)).toBe(true)
    }
  })

  it('网址片段随机拼接时也一样', () => {
    const rand = lcg(7)
    const parts = ['https://a.com/', 'http://b.org', '[x](', ')', '**', '_', '`', 'javascript:', ' ', '(', '\\', '~~']
    for (let n = 0; n < 2000; n++) {
      let src = ''
      for (let i = 0, k = Math.floor(rand() * 8); i < k; i++) src += parts[Math.floor(rand() * parts.length)]
      const nodes = parseInline(src)
      expect(inlineSource(nodes), JSON.stringify(src)).toBe(src)
      for (const link of links(nodes)) expect(isSafeHref(link.href), JSON.stringify(src)).toBe(true)
    }
  })
})

describe('parseInline：病态输入不会拖慢', () => {
  /** 在限定时间内解析完，返回耗时（毫秒） */
  function timeOf(src: string): number {
    const start = performance.now()
    inlineSource(parseInline(src))
    return performance.now() - start
  }

  it.each([
    ['大量没配对的星号', '*a '.repeat(7000)],
    ['大量没配对的反引号', '`a '.repeat(7000)],
    ['大量没配对的方括号', '[a '.repeat(7000)],
    ['大量下划线标签', '<u>a '.repeat(5000)],
    ['大量转义', '\\*'.repeat(10000)],
  ])('%s（两万字）在 1.5 秒内处理完', (_name, src) => {
    expect(timeOf(src)).toBeLessThan(1500)
  })
})
