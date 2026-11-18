import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppRoutes } from '@/app/AppRoutes'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { installFakeApi } from '@/test/fakeApi'
import { makeBoard, makeSnapshot } from '@/test/factories'
import { renderApp } from '@/test/renderApp'

afterEach(() => vi.unstubAllGlobals())

// 显示当前地址，用来断言跳转
function LocationProbe() {
  return <output data-testid="location">{useLocation().pathname}</output>
}

const renderPage = () =>
  renderApp(
    <>
      <AppRoutes />
      <LocationProbe />
    </>,
  )

describe('BoardListPage', () => {
  it('先显示加载占位，再列出所有看板：编号前缀、名称、说明、项目路径，并链接到看板页', async () => {
    installFakeApi({
      'GET /api/v1/boards': [
        makeBoard({ id: 1, name: 'Slate', slug: 'SLATE', description: '看板本身', project_path: '/code/slate' }),
        makeBoard({ id: 2, name: '官网', slug: 'WEB' }),
      ],
    })
    renderPage()
    expect(screen.getByText(en.boards.subtitle).closest('div')).toBeInTheDocument()
    const link = (await screen.findByText('看板本身')).closest('a') as HTMLElement
    expect(link).toHaveAttribute('href', '/b/1')
    expect(within(link).getByText('SLATE')).toBeInTheDocument()
    expect(within(link).getByText('看板本身')).toBeInTheDocument()
    expect(within(link).getByText('/code/slate')).toBeInTheDocument()
    const second = screen.getByText('官网').closest('a') as HTMLElement
    expect(within(second).getByText(en.boards.noPath)).toBeInTheDocument()
  })

  it('没有看板时显示引导，点按钮打开新建对话框', async () => {
    installFakeApi({ 'GET /api/v1/boards': [] })
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: en.boards.empty.cta }))
    expect(await screen.findByRole('dialog')).toHaveTextContent(en.boards.create.title)
  })

  it('加载失败显示原因和重试按钮，重试后恢复', async () => {
    const api = installFakeApi({ 'GET /api/v1/boards': () => ({ status: 500, body: { reason: 'ErrStore' } }) })
    renderPage()
    expect(await screen.findByRole('alert')).toHaveTextContent(en.boards.loadFailed)
    expect(screen.getByRole('alert')).toHaveTextContent(en.errors.ErrStore)
    api.set('GET /api/v1/boards', [makeBoard({ id: 1, name: '恢复了' })])
    await userEvent.click(screen.getByRole('button', { name: en.common.retry }))
    expect(await screen.findByText('恢复了')).toBeInTheDocument()
  })
})

describe('新建看板', () => {
  const open = async () => {
    const api = installFakeApi({ 'GET /api/v1/boards': [makeBoard({ id: 1, name: 'Existing' })] })
    renderPage()
    await userEvent.click(await screen.findByRole('button', { name: en.boards.new }))
    return { api, dialog: await screen.findByRole('dialog') }
  }

  it('名称为空时提示并且不发请求', async () => {
    const { api, dialog } = await open()
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(en.boards.create.errors.nameRequired)
    expect(api.count('POST /api/v1/boards')).toBe(0)
  })

  it('编号前缀和项目路径不合法时分别提示', async () => {
    const { api, dialog } = await open()
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'X')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.slug)), '1bad')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.projectPath)), 'relative')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    const alerts = await within(dialog).findAllByRole('alert')
    expect(alerts.map((a) => a.textContent)).toEqual([
      en.boards.create.errors.slugInvalid,
      en.boards.create.errors.pathInvalid,
    ])
    expect(api.count('POST /api/v1/boards')).toBe(0)
  })

  it('提交：编号前缀自动大写，名称去掉首尾空白，带上模板和当前界面语言，成功后跳到新看板', async () => {
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', makeSnapshot({ id: 7 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), '  My App  ')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.slug)), 'myapp')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.projectPath)), '/code/app')
    await userEvent.click(within(dialog).getByLabelText(new RegExp(en.boards.create.templates.simple.name)))
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/b/7'))
    // 创建成功后看板列表缓存失效，切换菜单里能看到新看板
    await waitFor(() => expect(api.count('GET /api/v1/boards')).toBe(2))
    const post = api.calls.find((c) => c.method === 'POST')
    expect(post?.body).toEqual({ name: 'My App', slug: 'MYAPP', project_path: '/code/app', template: 'simple', locale: 'en' })
  })

  it('只填名称时，可选字段不会作为空串提交；默认模板是研发流程', async () => {
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', makeSnapshot({ id: 8 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Solo')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toEqual({ name: 'Solo', template: 'dev', locale: 'en' })
  })

  it('服务端拒绝（编号前缀被占用）：提示原因，对话框保持打开，输入不丢', async () => {
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', () => ({ status: 409, body: { reason: 'ErrBoardSlugTaken', msg: '编号前缀已被占用' } }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Dup')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    expect(await screen.findByText(en.errors.ErrBoardSlugTaken)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(within(dialog).getByLabelText(en.boards.create.name)).toHaveValue('Dup')
  })

  it('界面是日文时，提交的 locale 是 ja', async () => {
    const { api, dialog } = await open()
    const { setLanguage } = await import('@/i18n')
    await setLanguage('ja')
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(ja.boards.create.name), 'JP')
    await userEvent.click(within(dialog).getByRole('button', { name: ja.boards.create.submit }))
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toMatchObject({ locale: 'ja' })
  })

  it('取消关闭对话框，重新打开是一份空白表单', async () => {
    const { dialog } = await open()
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'draft')
    await userEvent.click(within(dialog).getByRole('button', { name: en.common.cancel }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await userEvent.click(screen.getByRole('button', { name: en.boards.new }))
    expect(within(await screen.findByRole('dialog')).getByLabelText(en.boards.create.name)).toHaveValue('')
  })
})
