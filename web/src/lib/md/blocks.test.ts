import { describe, expect, it } from 'vitest'
import { classifyLines, parseBlocks, toggleTaskAt, type Block, type ListItem } from './blocks'

const classify = (src: string) => classifyLines(src.split('\n'))
const kinds = (src: string) => classify(src).map((l) => l.kind)

/** 把块树压成一行好读的字符串，方便在用例里比较结构 */
function show(blocks: readonly Block[]): string {
  return blocks
    .map((b) => {
      switch (b.kind) {
        case 'code':
          return `code(${b.info}|${b.text})`
        case 'heading':
          return `h${b.level}(${b.text})`
        case 'hr':
          return 'hr'
        case 'quote':
          return `quote[${show(b.children)}]`
        case 'list':
          return `${b.ordered ? `ol${b.start}` : 'ul'}[${b.items.map(showItem).join(',')}]`
        case 'p':
          return `p(${b.text})`
      }
    })
    .join(' ')
}

/** 列表项：任务项前面带 ☐ 或 ☑，有子块时用花括号括起来 */
function showItem(item: ListItem): string {
  const box = item.checked === undefined ? '' : item.checked ? '☑' : '☐'
  const kids = item.children.length > 0 ? `{${show(item.children)}}` : ''
  return `${box}${item.text}${kids}`
}

describe('classifyLines：每一行属于哪一种', () => {
  it('标题 1 到 6 级，必须有空格；七个井号和没有空格的不是标题', () => {
    expect(classify('# a')[0]).toMatchObject({ kind: 'heading', level: 1, markLen: 2 })
    expect(classify('###### f')[0]).toMatchObject({ kind: 'heading', level: 6, markLen: 7 })
    expect(classify('##  两个空格')[0]).toMatchObject({ kind: 'heading', level: 2, markLen: 4 })
    expect(classify('# ')[0]).toMatchObject({ kind: 'heading', level: 1, markLen: 2 })
    expect(kinds('####### x\n#tag\n#')).toEqual(['para', 'para', 'para'])
  })

  it('空行和只有空白的行都是 blank', () => {
    expect(kinds('a\n\n   \n\t\nb')).toEqual(['para', 'blank', 'blank', 'blank', 'para'])
  })

  it('围栏代码：开头、里面、结尾；里面的标题、列表都只是代码', () => {
    expect(kinds('```js\n# not heading\n- not item\n```\nafter')).toEqual(['fence', 'code', 'code', 'fence', 'para'])
    expect(classify('```js\nx\n```')[0].fence).toEqual({ open: true, char: '`', len: 3, info: 'js' })
    expect(classify('```js\nx\n```')[2].fence).toMatchObject({ open: false })
  })

  it('围栏没有结束就一直到文末；波浪线围栏；结尾围栏要同种符号且不短于开头', () => {
    expect(kinds('```\na\nb')).toEqual(['fence', 'code', 'code'])
    expect(kinds('~~~\na\n~~~')).toEqual(['fence', 'code', 'fence'])
    expect(kinds('````\n```\nx\n````')).toEqual(['fence', 'code', 'code', 'fence'])
    expect(kinds('```\n~~~\n```')).toEqual(['fence', 'code', 'fence'])
  })

  it('反引号围栏的说明里不能再有反引号，否则不是围栏', () => {
    expect(kinds('```a`b')).toEqual(['para'])
  })

  it('分隔线：三个以上的 - * _，中间可有空格', () => {
    expect(kinds('---\n***\n___\n- - -\n--')).toEqual(['hr', 'hr', 'hr', 'hr', 'para'])
  })

  it('引用：> 后面空格可有可无，层数按 > 的个数', () => {
    expect(classify('> a')[0]).toMatchObject({ kind: 'quote', level: 1, markLen: 2 })
    expect(classify('>a')[0]).toMatchObject({ kind: 'quote', level: 1, markLen: 1 })
    expect(classify('> > b')[0]).toMatchObject({ kind: 'quote', level: 2, markLen: 4 })
  })

  it('无序、有序、缩进的列表项，记下缩进、记号长度和编号', () => {
    expect(classify('- a')[0]).toMatchObject({ kind: 'item', ordered: false, indent: 0, markLen: 2 })
    expect(classify('* a')[0]).toMatchObject({ kind: 'item', ordered: false })
    expect(classify('+ a')[0]).toMatchObject({ kind: 'item', ordered: false })
    expect(classify('12. a')[0]).toMatchObject({ kind: 'item', ordered: true, number: 12, markLen: 4 })
    expect(classify('1) a')[0]).toMatchObject({ kind: 'item', ordered: true, number: 1 })
    expect(classify('    - b')[0]).toMatchObject({ kind: 'item', indent: 4, indentLen: 4, markLen: 6 })
    expect(classify('\t- b')[0]).toMatchObject({ kind: 'item', indent: 4, indentLen: 1, markLen: 3 })
  })

  it('任务项：方框状态和记号长度；方框后必须有空格', () => {
    expect(classify('- [ ] t')[0]).toMatchObject({ kind: 'item', checked: false, bulletEnd: 2, markLen: 6 })
    expect(classify('- [x] t')[0]).toMatchObject({ checked: true })
    expect(classify('- [X] t')[0]).toMatchObject({ checked: true })
    expect(classify('- [ ]t')[0].checked).toBeUndefined()
    expect(classify('- [] t')[0].checked).toBeUndefined()
    expect(classify('- plain')[0].checked).toBeUndefined()
  })

  it('"- " 带空格是空列表项；"-x" 和单独的 "-" 是段落', () => {
    expect(classify('- ')[0]).toMatchObject({ kind: 'item', markLen: 2 })
    expect(kinds('-x\n-')).toEqual(['para', 'para'])
  })

  it('紧跟在段落后面：无序项可以打断段落，有序项只有从 1 开始才行，空项不行', () => {
    expect(kinds('text\n- b')).toEqual(['para', 'item'])
    expect(kinds('text\n1. b')).toEqual(['para', 'item'])
    expect(kinds('text\n2. b')).toEqual(['para', 'para'])
    expect(kinds('text\n- ')).toEqual(['para', 'para'])
    expect(kinds('\n2. b')).toEqual(['blank', 'item'])
  })

  it('每行一个结果，数量和输入一致', () => {
    expect(classify('a\nb\n\n').length).toBe(4)
  })
})

describe('parseBlocks：块结构', () => {
  it.each([
    ['空输入', '', ''],
    ['只有空行', '\n\n  \n', ''],
    ['段落按空行分开，软换行保留', '# T\n\nhello\nworld\n\nbye', 'h1(T) p(hello\nworld) p(bye)'],
    ['CRLF 当作换行', 'a\r\nb', 'p(a\nb)'],
    ['段落首的空格去掉', '  a\n  b', 'p(a\nb)'],
    ['标题 1 到 6 级', '# a\n## b\n###### f', 'h1(a) h2(b) h6(f)'],
    ['七个井号是段落', '####### x', 'p(####### x)'],
    ['标题打断段落', 'text\n## h\nmore', 'p(text) h2(h) p(more)'],
    ['分隔线', 'a\n\n---\n\nb', 'p(a) hr p(b)'],
    ['紧跟段落的 --- 也是分隔线（不支持 setext 标题）', 'a\n---\nb', 'p(a) hr p(b)'],
    ['围栏代码保留空行和井号', '```js\nconst a = 1\n\n# not heading\n```\nafter', 'code(js|const a = 1\n\n# not heading) p(after)'],
    ['围栏没结束就到文末', '```\nabc', 'code(|abc)'],
    ['长围栏里可以有短围栏', '````\n```\nx\n````', 'code(|```\nx)'],
    ['围栏打断段落', 'text\n```\ncode\n```', 'p(text) code(|code)'],
    ['引用打断段落', 'text\n> q', 'p(text) quote[p(q)]'],
    ['引用里的多行合成一个段落', '> a\n> b\n\nc', 'quote[p(a\nb)] p(c)'],
    ['引用里可以有标题和列表', '> # h\n> - x', 'quote[h1(h) ul[x]]'],
    ['引用嵌套', '> > a', 'quote[quote[p(a)]]'],
  ])('%s', (_name, src, want) => {
    expect(show(parseBlocks(src))).toBe(want)
  })
})

describe('parseBlocks：列表', () => {
  it.each([
    ['无序列表', '- a\n- b', 'ul[a,b]'],
    ['* 和 + 也行', '* a\n+ b', 'ul[a,b]'],
    ['有序列表记下起始编号', '3. a\n4. b', 'ol3[a,b]'],
    ['两层嵌套（两空格）', '- a\n  - b\n- c', 'ul[a{ul[b]},c]'],
    ['三层嵌套', '- a\n  - b\n    - c\n- d', 'ul[a{ul[b{ul[c]}]},d]'],
    ['四空格缩进也是一层', '- a\n    - b', 'ul[a{ul[b]}]'],
    ['缩进用 Tab', '- a\n\t- b', 'ul[a{ul[b]}]'],
    ['有序里嵌无序', '1. a\n   - b\n2. c', 'ol1[a{ul[b]},c]'],
    ['缩进回退到更外层', '- a\n  - b\n    - c\n  - d\n- e', 'ul[a{ul[b{ul[c]},d]},e]'],
    ['缩进不整齐也能嵌套', '- a\n   - b\n - c', 'ul[a{ul[b]},c]'],
    ['无序紧跟有序是两个列表', '- a\n1. b', 'ul[a] ol1[b]'],
    ['有序列表里带续行，后面的 2. 仍是同一个列表', '1. a\n   cont\n2. b', 'ol1[a\ncont,b]'],
    ['缩进的续行并入上一项', '- a\n  more\n- b', 'ul[a\nmore,b]'],
    ['条目之间有空行仍是同一个列表', '- a\n\n- b', 'ul[a,b]'],
    ['空行后不是列表项就结束列表', '- a\n\ntext', 'ul[a] p(text)'],
    ['没缩进的下一行结束列表', '- a\ntext', 'ul[a] p(text)'],
    ['列表打断段落', 'intro:\n- a\n- b', 'p(intro:) ul[a,b]'],
    ['有序项从 1 开始才能打断段落', 'see\n1. foo', 'p(see) ol1[foo]'],
    ['有序项不从 1 开始不打断段落', 'see\n2. foo', 'p(see\n2. foo)'],
    ['任务项', '- [ ] a\n- [x] b', 'ul[☐a,☑b]'],
    ['任务项可以嵌套', '- [ ] a\n  - [x] b', 'ul[☐a{ul[☑b]}]'],
    ['空列表项', '- \n- b', 'ul[,b]'],
  ])('%s', (_name, src, want) => {
    expect(show(parseBlocks(src))).toBe(want)
  })

  it('每个列表项记着自己在原文里是第几行（从 0 数），引用里的也一样', () => {
    const [, list] = parseBlocks('x\n\n- [ ] a\n- [x] b')
    expect(list.kind === 'list' && list.items.map((i) => i.line)).toEqual([2, 3])
    const [quote] = parseBlocks('> - [ ] a')
    expect(quote.kind === 'quote' && quote.children[0].kind === 'list' && quote.children[0].items[0].line).toBe(0)
  })

  it('嵌套的深度有上限：几千层的引用、几百层的列表既不抛错也不丢字，层数被压在上限内', () => {
    const deepQuote = `${'> '.repeat(3000)}x`
    expect(() => parseBlocks(deepQuote)).not.toThrow()
    expect(show(parseBlocks(deepQuote))).toContain('x')
    expect(depthOf(parseBlocks(deepQuote))).toBeLessThanOrEqual(10)

    const deepList = Array.from({ length: 400 }, (_, i) => `${' '.repeat(i * 2)}- n${i}`).join('\n')
    expect(() => parseBlocks(deepList)).not.toThrow()
    expect(show(parseBlocks(deepList))).toContain('n399')
    expect(depthOf(parseBlocks(deepList))).toBeLessThanOrEqual(10)
  })
})

/** 块树的最大嵌套层数（引用和列表都算一层） */
function depthOf(blocks: readonly Block[]): number {
  let max = 0
  for (const b of blocks) {
    if (b.kind === 'quote') max = Math.max(max, 1 + depthOf(b.children))
    if (b.kind === 'list') for (const item of b.items) max = Math.max(max, 1 + depthOf(item.children))
  }
  return max
}

describe('toggleTaskAt', () => {
  it('把指定行的任务项在未完成和已完成之间切换', () => {
    expect(toggleTaskAt('- [ ] a\n- [x] b', 0)).toBe('- [x] a\n- [x] b')
    expect(toggleTaskAt('- [ ] a\n- [x] b', 1)).toBe('- [ ] a\n- [ ] b')
    expect(toggleTaskAt('- [X] a', 0)).toBe('- [ ] a')
    expect(toggleTaskAt('  - [ ] nested', 0)).toBe('  - [x] nested')
    expect(toggleTaskAt('1. [ ] ordered', 0)).toBe('1. [x] ordered')
    expect(toggleTaskAt('> - [ ] quoted', 0)).toBe('> - [x] quoted')
  })

  it('不是任务项、越界、围栏代码里的行都原样返回', () => {
    expect(toggleTaskAt('- plain', 0)).toBe('- plain')
    expect(toggleTaskAt('- [ ] a', 5)).toBe('- [ ] a')
    expect(toggleTaskAt('- [ ] a', -1)).toBe('- [ ] a')
    expect(toggleTaskAt('```\n- [ ] a\n```', 1)).toBe('```\n- [ ] a\n```')
    expect(toggleTaskAt('text [ ] not task', 0)).toBe('text [ ] not task')
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

describe('随机文档下的不变量', () => {
  const vocabulary = ['# h', '## h2', '- a', '  - b', '    - c', '1. n', '2. m', '> q', '> > qq', '```', '```js', '~~~', '---', '', '   ', 'text', '- [ ] t', '- [x] d', '  cont', '\t- tab', '*', '-', '>']

  it('分类的行数和输入一致；解析从不抛错；任务切换两次回到原文', () => {
    const rand = lcg(42)
    for (let n = 0; n < 1500; n++) {
      const lines: string[] = []
      for (let i = 0, k = Math.floor(rand() * 14); i < k; i++) lines.push(vocabulary[Math.floor(rand() * vocabulary.length)])
      const src = lines.join('\n')
      expect(classifyLines(lines).length, JSON.stringify(src)).toBe(lines.length)
      expect(() => parseBlocks(src), JSON.stringify(src)).not.toThrow()
      const at = Math.floor(rand() * Math.max(1, lines.length))
      expect(toggleTaskAt(toggleTaskAt(src, at), at), JSON.stringify(src)).toBe(src)
    }
  })
})
