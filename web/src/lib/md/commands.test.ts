import { describe, expect, it } from 'vitest'
import {
  deleteBackward,
  deleteForward,
  indentList,
  insertCodeBlock,
  insertDivider,
  insertLink,
  insertNewline,
  insertText,
  nextGrapheme,
  pasteText,
  prevGrapheme,
  setHeading,
  toggleList,
  toggleQuote,
  toggleWrap,
  type EditState,
} from './commands'

/**
 * 用带记号的字符串写用例：‸ 是光标；⟦…⟧ 是选区（光标在右端）；⟪…⟫ 是反向选区（光标在左端）。
 */
function st(src: string): EditState {
  let value = ''
  let anchor = -1
  let head = -1
  for (const ch of src) {
    if (ch === '‸') anchor = head = value.length
    else if (ch === '⟦' || ch === '⟫') anchor = value.length
    else if (ch === '⟧' || ch === '⟪') head = value.length
    else value += ch
  }
  if (anchor < 0 || head < 0) throw new Error(`用例缺少光标记号：${src}`)
  return { value, anchor, head }
}

/** st 的反过程：把结果写回带记号的字符串，方便和期望值直接比 */
function show(s: EditState): string {
  const lo = Math.min(s.anchor, s.head)
  const hi = Math.max(s.anchor, s.head)
  if (lo === hi) return `${s.value.slice(0, lo)}‸${s.value.slice(lo)}`
  const [open, close] = s.anchor < s.head ? ['⟦', '⟧'] : ['⟪', '⟫']
  return `${s.value.slice(0, lo)}${open}${s.value.slice(lo, hi)}${close}${s.value.slice(hi)}`
}

/** 跑一条命令并把结果（或 null）转成字符串 */
const run = (fn: (s: EditState) => EditState | null, input: string): string | null => {
  const out = fn(st(input))
  return out === null ? null : show(out)
}

describe('记号辅助函数自检', () => {
  it('st 和 show 互为逆运算，反向选区也对', () => {
    for (const src of ['a‸b', 'a⟦bc⟧d', 'a⟪bc⟫d']) expect(show(st(src))).toBe(src)
    expect(st('a⟪bc⟫d')).toEqual({ value: 'abcd', anchor: 3, head: 1 })
  })
})

describe('prevGrapheme / nextGrapheme', () => {
  it('按用户看到的一个字符前进后退：代理对、组合表情、组合符都算一个', () => {
    expect(prevGrapheme('ab', 2)).toBe(1)
    expect(nextGrapheme('ab', 0)).toBe(1)
    expect(prevGrapheme('😀', 2)).toBe(0)
    expect(nextGrapheme('😀', 0)).toBe(2)
    const family = 'a👨‍👩‍👧'
    expect(prevGrapheme(family, family.length)).toBe(1)
    expect(nextGrapheme(family, 1)).toBe(family.length)
    expect(prevGrapheme('e\u0301', 2)).toBe(0)
    expect(nextGrapheme('e\u0301', 0)).toBe(2)
  })

  it('没有 Segmenter 时按码点退化，不会把代理对劈开', () => {
    expect(prevGrapheme('a😀', 3, null)).toBe(1)
    expect(nextGrapheme('😀a', 0, null)).toBe(2)
    expect(prevGrapheme('ab', 0, null)).toBe(0)
    expect(nextGrapheme('ab', 2, null)).toBe(2)
  })
})

describe('deleteBackward', () => {
  it.each([
    ['删一个字', 'ab‸', 'a‸'],
    ['删选区', 'a⟦bc⟧d', 'a‸d'],
    ['文首什么也不做', '‸a', '‸a'],
    ['行首并到上一行', 'a\n‸b', 'a‸b'],
    ['上一行是空行时只删一个空行', 'a\n\n‸b', 'a\n‸b'],
    ['代理对一次删完', '😀‸', '‸'],
    ['组合表情一次删完', 'a👨‍👩‍👧‸', 'a‸'],
    ['列表项文字开头：去掉记号', '- ‸a', '‸a'],
    ['有序项', '1. ‸a', '‸a'],
    ['任务项连方框一起去掉', '- [ ] ‸a', '‸a'],
    ['嵌套项先退一级', '- a\n  - ‸b', '- a\n- ‸b'],
    ['空列表项', '- ‸', '‸'],
    ['标题', '## ‸a', '‸a'],
    ['引用', '> ‸a', '‸a'],
    ['两层引用先退一层', '> > ‸a', '> ‸a'],
    ['记号里面按普通字符删', '#‸# a', '‸# a'],
    ['分隔线后面的行首：整条分隔线删掉', 'a\n---\n‸b', 'a\n‸b'],
    ['围栏后面的文字行首不并进围栏', '```\nx\n```\n‸b', '```\nx\n```\n‸b'],
    ['代码块第一行行首不并进开头围栏', '```\n‸x\n```', '```\n‸x\n```'],
    ['结尾围栏行首不并进代码', '```\nx\n‸```', '```\nx\n‸```'],
    ['围栏后面的空行可以并回去', '```\nx\n```\n‸', '```\nx\n```‸'],
    ['续行文字开头并到上一行', '- a\n  ‸b', '- a‸b'],
  ])('%s', (_name, input, want) => {
    expect(run((s) => deleteBackward(s), input)).toBe(want)
  })

  it('按词删：连同后面的空格一起', () => {
    expect(run((s) => deleteBackward(s, 'word'), 'hello world‸')).toBe('hello ‸')
    expect(run((s) => deleteBackward(s, 'word'), 'hello ‸')).toBe('‸')
    expect(run((s) => deleteBackward(s, 'word'), 'a\n‸b')).toBe('a‸b')
  })

  it('按行删：删到正文开头，不动列表记号', () => {
    expect(run((s) => deleteBackward(s, 'line'), 'abc def‸')).toBe('‸')
    expect(run((s) => deleteBackward(s, 'line'), '- abc‸')).toBe('- ‸')
    expect(run((s) => deleteBackward(s, 'line'), '- ‸abc')).toBe('‸abc')
  })
})

describe('deleteForward', () => {
  it.each([
    ['删一个字', '‸ab', '‸b'],
    ['删选区', 'a⟦bc⟧d', 'a‸d'],
    ['文末什么也不做', 'a‸', 'a‸'],
    ['行尾并入下一行', 'a‸\nb', 'a‸b'],
    ['并入时去掉下一行的列表记号', 'a‸\n- b', 'a‸b'],
    ['并入时去掉下一行的标题记号', 'a‸\n## b', 'a‸b'],
    ['下一行是分隔线时整条删掉', 'a‸\n---\nb', 'a‸\nb'],
    ['不把文字并进围栏', 'a‸\n```\nx\n```', 'a‸\n```\nx\n```'],
    ['代码行尾不并进结尾围栏', '```\nx‸\n```', '```\nx‸\n```'],
    ['代理对一次删完', '‸😀', '‸'],
    ['空行后面接列表项：只删空行，不拆列表', '‸\n- b', '‸- b'],
  ])('%s', (_name, input, want) => {
    expect(run((s) => deleteForward(s), input)).toBe(want)
  })

  it('按词删和按行删', () => {
    expect(run((s) => deleteForward(s, 'word'), '‸hello world')).toBe('‸ world')
    expect(run((s) => deleteForward(s, 'line'), 'a‸bc\nd')).toBe('a‸\nd')
  })
})

describe('insertNewline', () => {
  it.each([
    ['段落末尾：另起一段', 'abc‸', 'abc\n\n‸'],
    ['段落中间：一分为二', 'ab‸c', 'ab\n\n‸c'],
    ['无序列表末尾：新条目', '- a‸', '- a\n- ‸'],
    ['无序列表中间：拆成两项', '- a‸b', '- a\n- ‸b'],
    ['保持记号符号', '* a‸', '* a\n* ‸'],
    ['保持缩进', '  - a‸', '  - a\n  - ‸'],
    ['有序列表编号加一', '1. a‸', '1. a\n2. ‸'],
    ['有序列表保持分隔符', '9) a‸', '9) a\n10) ‸'],
    ['任务项的新条目是未完成', '- [x] a‸', '- [x] a\n- [ ] ‸'],
    ['空条目退出列表', '- a\n- ‸', '- a\n‸'],
    ['空的嵌套条目退一级', '- a\n  - ‸', '- a\n- ‸'],
    ['空任务条目退出', '- [ ] ‸', '‸'],
    ['续行末尾也新开一项', '- a\n  b‸', '- a\n  b\n- ‸'],
    ['引用换行继续引用', '> a‸', '> a\n> ‸'],
    ['空引用行退出', '> a\n> ‸', '> a\n‸'],
    ['两层引用的空行退一层', '> > ‸', '> ‸'],
    ['标题末尾换成普通行', '# T‸', '# T\n‸'],
    ['标题中间拆开', '# T‸x', '# T\n‸x'],
    ['标题文字开头：在上方加空行', '# ‸T', '\n# ‸T'],
    ['空标题退出', '## ‸', '‸'],
    ['围栏开头自动补结尾', '```go‸', '```go\n‸\n```'],
    ['已有结尾的围栏只换行', '```go‸\nx\n```', '```go\n‸\nx\n```'],
    ['代码里的换行保持缩进', '```\n  a‸\n```', '```\n  a\n  ‸\n```'],
    ['分隔线后换行', '---‸', '---\n‸'],
    ['空行再按回车', 'a\n‸', 'a\n\n‸'],
    ['缩进不够的文字不算列表续行', '- a\nb‸', '- a\nb\n\n‸'],
    ['空行之后的缩进文字不算列表续行', '- a\n\n  b‸', '- a\n\n  b\n\n‸'],
    ['替换选区', 'a⟦bc⟧d', 'a\n\n‸d'],
  ])('%s', (_name, input, want) => {
    expect(run((s) => insertNewline(s), input)).toBe(want)
  })

  it.each([
    ['普通段落只换行', 'ab‸c', 'ab\n‸c'],
    ['列表里换成对齐的续行', '- a‸b', '- a\n  ‸b'],
    ['有序列表的续行按记号宽度对齐', '10. a‸b', '10. a\n    ‸b'],
    ['引用里继续引用', '> a‸b', '> a\n> ‸b'],
    ['续行里保持同样的缩进', '- a\n  b‸c', '- a\n  b\n  ‸c'],
    ['标题里只换行', '# T‸x', '# T\n‸x'],
  ])('Shift+Enter：%s', (_name, input, want) => {
    expect(run((s) => insertNewline(s, true), input)).toBe(want)
  })
})

describe('toggleWrap', () => {
  const bold = (input: string) => run((s) => toggleWrap(s, 'bold'), input)

  it.each([
    ['选中的文字加粗', 'a⟦bc⟧d', 'a**⟦bc⟧**d'],
    ['没有选区：插入一对记号，光标在中间', 'ab‸', 'ab**‸**'],
    ['空的一对记号上再按：去掉', 'a**‸**b', 'a‸b'],
    ['选的正好是粗体里的字：取消', 'a**⟦bc⟧**d', 'a⟦bc⟧d'],
    ['选区连记号一起选：取消', 'a⟦**bc**⟧d', 'a⟦bc⟧d'],
    ['光标在粗体里：整段取消', '**a‸b**', 'a‸b'],
    ['只选粗体里的一部分：拆开', '**a⟦b⟧c**', '**a**⟦b⟧**c**'],
    ['选在粗体开头：只留后面的', '**⟦a⟧bc**', '⟦a⟧**bc**'],
    ['选在粗体结尾：只留前面的', '**ab⟦c⟧**', '**ab**⟦c⟧'],
    ['保持反向选区', 'a⟪bc⟫d', 'a**⟪bc⟫**d'],
    ['两端有空格时记号留在空格外面', '⟦ a ⟧', ' **⟦a⟧** '],
    ['多行选区处理后框住各行的正文', '⟦ a\n b ⟧', ' **⟦a**\n **b⟧** '],
    ['全是空白不处理', '⟦  ⟧', '⟦  ⟧'],
    ['斜体里再加粗是嵌套', '*⟦a⟧*', '***⟦a⟧***'],
    ['多行选区逐行加', '⟦a\nb⟧', '**⟦a**\n**b⟧**'],
    ['多行全是粗体就逐行取消', '⟦**a**\n**b**⟧', '⟦a\nb⟧'],
    ['多行跳过空行', '⟦a\n\nb⟧', '**⟦a**\n\n**b⟧**'],
    ['列表项的记号不被包进去', '⟦- a\n- b⟧', '- **⟦a**\n- **b⟧**'],
    ['多行里有的已经是粗体：只给没加的加', '⟦**a**\nb⟧', '⟦**a**\n**b⟧**'],
    ['代码里的行不加格式', '```\n⟦x⟧\n```', '```\n⟦x⟧\n```'],
  ])('粗体：%s', (_name, input, want) => {
    expect(bold(input)).toBe(want)
  })

  it.each([
    ['italic', 'a⟦b⟧c', 'a*⟦b⟧*c'],
    ['strike', 'a⟦b⟧c', 'a~~⟦b⟧~~c'],
    ['underline', 'a⟦b⟧c', 'a<u>⟦b⟧</u>c'],
    ['code', 'a⟦b⟧c', 'a`⟦b⟧`c'],
  ] as const)('%s 用各自的记号', (mark, input, want) => {
    expect(run((s) => toggleWrap(s, mark), input)).toBe(want)
  })

  it.each([
    ['italic', 'a*⟦b⟧*c', 'a⟦b⟧c'],
    ['strike', 'a~~⟦b⟧~~c', 'a⟦b⟧c'],
    ['underline', 'a<u>⟦b⟧</u>c', 'a⟦b⟧c'],
    ['code', 'a`⟦b⟧`c', 'a⟦b⟧c'],
  ] as const)('%s 再按一次取消', (mark, input, want) => {
    expect(run((s) => toggleWrap(s, mark), input)).toBe(want)
  })

  it('斜体加在粗体里面，粗体里的斜体不会被误当成粗体取消', () => {
    expect(run((s) => toggleWrap(s, 'italic'), '**a⟦b⟧c**')).toBe('**a*⟦b⟧*c**')
    expect(bold('*a⟦b⟧c*')).toBe('*a**⟦b⟧**c*')
  })

  it('代码里带反引号时不处理，免得生成坏的代码段', () => {
    expect(run((s) => toggleWrap(s, 'code'), '⟦a`b⟧')).toBe('⟦a`b⟧')
  })
})

describe('setHeading', () => {
  it.each([
    ['空行也能设标题', '‸', 1, '# ‸'],
    ['光标跟着正文移动', 'a‸bc', 1, '# a‸bc'],
    ['换级别', '## a‸', 3, '### a‸'],
    ['同级别再按：取消', '## a‸', 2, 'a‸'],
    ['0 级就是正文', '## a‸', 0, 'a‸'],
    ['0 级不动非标题行', '- a‸', 0, '- a‸'],
    ['列表项变标题，记号被替换', '- a‸', 1, '# a‸'],
    ['引用变标题', '> a‸', 1, '# a‸'],
    ['多行逐行设置', '⟦a\nb⟧', 1, '# ⟦a\n# b⟧'],
    ['多行跳过空行', '⟦a\n\nb⟧', 2, '## ⟦a\n\n## b⟧'],
    ['代码里的行不动', '```\n‸x\n```', 1, '```\n‸x\n```'],
    ['选区右端在下一行行首时，下一行不算', '⟦a\n⟧b', 1, '# ⟦a\n⟧b'],
    ['分隔线不能改成标题', '---‸', 1, '---‸'],
  ])('%s', (_name, input, level, want) => {
    expect(run((s) => setHeading(s, level), input)).toBe(want)
  })
})

describe('toggleList', () => {
  it.each([
    ['段落变无序', 'a‸', 'bullet', '- a‸'],
    ['无序再按：取消', '- a‸', 'bullet', 'a‸'],
    ['无序变有序', '- a‸', 'ordered', '1. a‸'],
    ['多行有序编号递增', '⟦a\nb\nc⟧', 'ordered', '1. ⟦a\n2. b\n3. c⟧'],
    ['段落变任务', 'a‸', 'task', '- [ ] a‸'],
    ['任务再按：取消', '- [ ] a‸', 'task', 'a‸'],
    ['无序变任务', '- a‸', 'task', '- [ ] a‸'],
    ['任务变无序', '- [x] a‸', 'bullet', '- a‸'],
    ['标题变列表', '# a‸', 'bullet', '- a‸'],
    ['缩进保留', '  - a‸', 'ordered', '  1. a‸'],
    ['空行插入记号', '‸', 'bullet', '- ‸'],
    ['多行跳过空行，编号不断', '⟦a\n\nb⟧', 'ordered', '1. ⟦a\n\n2. b⟧'],
    ['部分已是列表时全部统一', '⟦- a\nb⟧', 'bullet', '⟦- a\n- b⟧'],
  ] as const)('%s', (_name, input, kind, want) => {
    expect(run((s) => toggleList(s, kind), input)).toBe(want)
  })
})

describe('toggleQuote', () => {
  it.each([
    ['加引用', 'a‸', '> a‸'],
    ['再按取消', '> a‸', 'a‸'],
    ['两层引用只退一层', '> > a‸', '> a‸'],
    ['多行加引用', '⟦a\nb⟧', '> ⟦a\n> b⟧'],
    ['多行全是引用才取消', '⟦> a\n> b⟧', '⟦a\nb⟧'],
    ['部分是引用时全部再加一层', '⟦> a\nb⟧', '> ⟦> a\n> b⟧'],
    ['空行也能加', '‸', '> ‸'],
  ])('%s', (_name, input, want) => {
    expect(run((s) => toggleQuote(s), input)).toBe(want)
  })
})

describe('insertCodeBlock / insertDivider / insertLink', () => {
  it.each([
    ['空行：生成空代码块，光标在里面', '‸', '```\n‸\n```'],
    ['有文字的行整行包起来', 'a‸', '```\na‸\n```'],
    ['多行选区整体包起来', '⟦a\nb⟧', '```\n⟦a\nb⟧\n```'],
    ['已经在代码块里不处理', '```\n‸x\n```', '```\n‸x\n```'],
    ['正文里有三个反引号就用更长的围栏', '⟦a ``` b⟧', '````\n⟦a ``` b⟧\n````'],
  ])('代码块：%s', (_name, input, want) => {
    expect(run((s) => insertCodeBlock(s), input)).toBe(want)
  })

  it.each([
    ['在行后插入，光标落到下一行', 'a‸', 'a\n---\n‸'],
    ['空行直接变成分隔线', '‸', '---\n‸'],
    ['后面还有内容时空出一行', 'a‸\nb', 'a\n---\n‸\nb'],
    ['代码块里不处理', '```\n‸x\n```', '```\n‸x\n```'],
  ])('分隔线：%s', (_name, input, want) => {
    expect(run((s) => insertDivider(s), input)).toBe(want)
  })

  it.each([
    ['选中文字：括号留空，光标进括号', '⟦abc⟧', '[abc](‸)'],
    ['选中的是网址：作为地址，光标进方括号', '⟦https://a.com⟧', '[‸](https://a.com)'],
    ['没选中：生成空链接，光标在方括号里', '‸', '[‸]()'],
  ])('链接：%s', (_name, input, want) => {
    expect(run((s) => insertLink(s), input)).toBe(want)
  })
})

describe('indentList', () => {
  it.each([
    ['第二项缩进成子项', '- a\n- ‸b', false, '- a\n  - ‸b'],
    ['第一项不能缩进（原样返回）', '- ‸a', false, '- ‸a'],
    ['不能越级缩进', '- a\n  - ‸b', false, '- a\n  - ‸b'],
    ['多选一起缩进', '- a\n⟦- b\n- c⟧', false, '- a\n  ⟦- b\n  - c⟧'],
    ['有序列表也一样', '1. a\n2. ‸b', false, '1. a\n  2. ‸b'],
    ['非列表行返回 null', 'ab‸', false, null],
    ['退一级', '- a\n  - ‸b', true, '- a\n- ‸b'],
    ['顶层退不动（原样返回）', '- ‸a', true, '- ‸a'],
    ['一次只退两格', '- a\n    - ‸b', true, '- a\n  - ‸b'],
    ['非列表行退格返回 null', 'ab‸', true, null],
  ])('%s', (_name, input, outdent, want) => {
    expect(run((s) => indentList(s, outdent), input)).toBe(want)
  })
})

describe('insertText', () => {
  it.each([
    ['插入', 'a‸c', 'b', 100, 'ab‸c'],
    ['替换选区', 'a⟦b⟧c', 'X', 100, 'aX‸c'],
    ['超长截断', 'ab‸', 'cdef', 4, 'abcd‸'],
    ['文档已超上限时有选区也不删不插', 'abcd⟦e⟧', 'xyz', 3, 'abcd⟦e⟧'],
    ['代理对不被劈开', '‸', 'a😀', 2, 'a‸'],
    ['网址不会变成链接', '⟦字⟧', 'https://a.com', 100, 'https://a.com‸'],
  ])('%s', (_name, input, text, max, want) => {
    expect(show(insertText(st(input), text, max))).toBe(want)
  })
})

describe('pasteText', () => {
  it.each([
    ['插入', 'a‸', 'xyz', 100, 'axyz‸'],
    ['替换选区', 'a⟦b⟧c', 'X', 100, 'aX‸c'],
    ['统一换行符', '‸', 'a\r\nb\rc', 100, 'a\nb\nc‸'],
    ['超长截断', 'ab‸', 'cdef', 4, 'abcd‸'],
    ['已满不再插入', 'abcd‸', 'x', 4, 'abcd‸'],
    ['文档本来就超过上限也不再插入', 'abcdef‸', 'xyz', 4, 'abcdef‸'],
    ['代理对不被劈开', '‸', 'a😀', 2, 'a‸'],
    ['网址粘贴到选中文字上变成链接', '⟦文字⟧', 'https://a.com', 100, '[文字](https://a.com)‸'],
    ['没有选区的网址原样插入', '‸', 'https://a.com', 100, 'https://a.com‸'],
    ['多行选区不做链接', '⟦a\nb⟧', 'https://a.com', 100, 'https://a.com‸'],
  ])('%s', (_name, input, text, max, want) => {
    expect(show(pasteText(st(input), text, max))).toBe(want)
  })
})

/** 带种子的伪随机数，失败时能原样重现 */
function rng(seed: number): () => number {
  let x = seed >>> 0
  return () => {
    x = (Math.imul(x, 1664525) + 1013904223) >>> 0
    return x / 2 ** 32
  }
}

const MARK_PIECES = ['a', 'b', ' ', '  ', '\n', '\n', '- ', '1. ', '> ', '# ', '## ', '**', '*', '`', '~~', '<u>', '</u>', '```', '[ ] ', '---', '\t', '😀', '你好', '[', ']', '(', ')', 'https://a.com']
const PLAIN_LINES = ['abc', 'x y', '', '你好', 'hello world', '']

/** 用一组片段随机拼出文档，再随机取光标或选区 */
function randomState(next: () => number, pieces: readonly string[], joiner: string): EditState {
  const count = Math.floor(next() * 12)
  const value = Array.from({ length: count }, () => pieces[Math.floor(next() * pieces.length)]).join(joiner)
  const pick = () => safeIndex(value, Math.floor(next() * (value.length + 1)))
  const anchor = pick()
  const head = next() < 0.5 ? anchor : pick()
  return { value, anchor, head }
}

/** 光标不会落在一个代理对的两半之间，随机出来的位置也要避开 */
function safeIndex(value: string, index: number): number {
  const mid = index > 0 && index < value.length && /[\uDC00-\uDFFF]/.test(value[index]) && /[\uD800-\uDBFF]/.test(value[index - 1])
  return mid ? index - 1 : index
}

/** 选区起点落在紧跟文字行的空行上：这时插入空的列表记号会被当成段落里的普通文字（和 CommonMark 一致），不参与"再按一次回到原样" */
function blankRightAfterText(s: EditState): boolean {
  const lines = s.value.split('\n')
  const index = s.value.slice(0, Math.min(s.anchor, s.head)).split('\n').length - 1
  return lines[index] === '' && index > 0 && lines[index - 1].trim() !== ''
}

const ALL_COMMANDS: [string, (s: EditState) => EditState | null][] = [
  ['退格', (s) => deleteBackward(s)],
  ['按词退格', (s) => deleteBackward(s, 'word')],
  ['按行退格', (s) => deleteBackward(s, 'line')],
  ['Delete', (s) => deleteForward(s)],
  ['按词 Delete', (s) => deleteForward(s, 'word')],
  ['按行 Delete', (s) => deleteForward(s, 'line')],
  ['回车', (s) => insertNewline(s)],
  ['Shift+回车', (s) => insertNewline(s, true)],
  ...(['bold', 'italic', 'strike', 'underline', 'code'] as const).map((m): [string, (s: EditState) => EditState | null] => [m, (s) => toggleWrap(s, m)]),
  ...[0, 1, 2, 3, 4, 5, 6].map((l): [string, (s: EditState) => EditState | null] => [`标题${l}`, (s) => setHeading(s, l)]),
  ...(['bullet', 'ordered', 'task'] as const).map((k): [string, (s: EditState) => EditState | null] => [k, (s) => toggleList(s, k)]),
  ['引用', (s) => toggleQuote(s)],
  ['代码块', (s) => insertCodeBlock(s)],
  ['分隔线', (s) => insertDivider(s)],
  ['链接', (s) => insertLink(s)],
  ['Tab', (s) => indentList(s, false)],
  ['Shift+Tab', (s) => indentList(s, true)],
  ['粘贴多行', (s) => pasteText(s, 'x\r\n- y\n```', 50)],
  ['粘贴网址', (s) => pasteText(s, 'https://b.com', 50)],
]

describe('随机文档上的不变量', () => {
  it('任何命令都不抛错，选区始终在文档范围内，也不会冒出 \\r', () => {
    for (let seed = 1; seed <= 400; seed++) {
      const next = rng(seed)
      const state = randomState(next, MARK_PIECES, '')
      for (const [name, fn] of ALL_COMMANDS) {
        let out: EditState | null = null
        try {
          out = fn(state)
        } catch (err) {
          throw new Error(`种子 ${seed} 的命令「${name}」抛错：${String(err)}；输入 ${JSON.stringify(show(state))}`, { cause: err })
        }
        if (out === null) continue
        const ok = Number.isInteger(out.anchor) && Number.isInteger(out.head) && out.anchor >= 0 && out.head >= 0 && out.anchor <= out.value.length && out.head <= out.value.length
        if (!ok || out.value.includes('\r')) throw new Error(`种子 ${seed} 的命令「${name}」结果不合法：${JSON.stringify(out)}；输入 ${JSON.stringify(show(state))}`)
      }
    }
  })

  it('命令之外的文字一个字都不会丢：删除类命令只会变短，其余命令至少保留原有的字母', () => {
    for (let seed = 1; seed <= 300; seed++) {
      const state = randomState(rng(seed), MARK_PIECES, '')
      const letters = (text: string) => text.replace(/[^a-b你好😀]/gu, '')
      for (const [name, fn] of ALL_COMMANDS) {
        if (name.includes('退格') || name.includes('Delete') || name.startsWith('粘贴')) continue
        // 回车会替换选中的文字，所以这两条命令用没有选区的状态来比
        const input = name.includes('回车') ? { ...state, anchor: state.head } : state
        const out = fn(input)
        if (out) expect(letters(out.value), `种子 ${seed} 命令「${name}」`).toBe(letters(state.value))
      }
    }
  })

  it('普通文字上：标题、列表、引用、行内格式再按一次就回到原样', () => {
    const toggles: [string, (s: EditState) => EditState][] = [
      ['标题', (s) => setHeading(s, 2)],
      ['无序', (s) => toggleList(s, 'bullet')],
      ['有序', (s) => toggleList(s, 'ordered')],
      ['任务', (s) => toggleList(s, 'task')],
      ['引用', (s) => toggleQuote(s)],
      ['粗体', (s) => toggleWrap(s, 'bold')],
      ['斜体', (s) => toggleWrap(s, 'italic')],
      ['删除线', (s) => toggleWrap(s, 'strike')],
      ['下划线', (s) => toggleWrap(s, 'underline')],
      ['代码', (s) => toggleWrap(s, 'code')],
    ]
    for (let seed = 1; seed <= 300; seed++) {
      const state = randomState(rng(seed), PLAIN_LINES, '\n')
      for (const [name, fn] of toggles) {
        if (['无序', '有序', '任务'].includes(name) && blankRightAfterText(state)) continue
        const once = fn(state)
        const twice = fn(once)
        expect(twice.value, `种子 ${seed}「${name}」输入 ${JSON.stringify(show(state))}`).toBe(state.value)
      }
    }
  })

  it('普通文字上：Shift+回车之后立刻退格，回到原样', () => {
    for (let seed = 1; seed <= 200; seed++) {
      const state = randomState(rng(seed), PLAIN_LINES.filter((l) => l !== ''), ' ')
      const collapsed = { ...state, anchor: state.head }
      const back = deleteBackward(insertNewline(collapsed, true))
      expect(back.value).toBe(state.value)
      expect(back.head).toBe(state.head)
    }
  })

  it('粘贴后文档总长度不超过上限', () => {
    for (let seed = 1; seed <= 200; seed++) {
      const state = randomState(rng(seed), MARK_PIECES, '')
      const max = Math.max(state.value.length, 5)
      const out = pasteText(state, 'x\n- y 😀 https://c.com', max)
      expect(out.value.length).toBeLessThanOrEqual(max)
    }
  })
})
