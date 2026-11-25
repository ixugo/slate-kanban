import { act, fireEvent, screen, waitFor, within } from '@testing-library/react'
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
  it('先显示加载占位，再列出所有项目：名称、说明（不显示编号前缀和项目路径），并链接到项目页', async () => {
    installFakeApi({
      'GET /api/v1/boards': [
        makeBoard({ id: 1, name: 'Slate', slug: 'SLATE', description: '看板本身', project_path: '/code/slate' }),
        makeBoard({ id: 2, name: '官网', slug: 'WEB' }),
      ],
    })
    renderPage()
    expect(screen.getByRole('heading', { level: 1, name: en.boards.title })).toBeInTheDocument()
    const link = (await screen.findByText('看板本身')).closest('a') as HTMLElement
    expect(link).toHaveAttribute('href', '/b/1')
    expect(within(link).queryByText('SLATE')).not.toBeInTheDocument()
    expect(within(link).getByText('看板本身')).toBeInTheDocument()
    // 工作目录只在创建和设置表单展示，不占项目列表空间
    expect(within(link).queryByText('/code/slate')).not.toBeInTheDocument()
  })

  it('“更新于”按最后活动时间显示，而不是项目自身的更新时间', async () => {
    const now = Date.now()
    installFakeApi({
      'GET /api/v1/boards': [
        makeBoard({
          id: 1,
          name: '活跃项目',
          updated_at: new Date(now - 3 * 86_400_000).toISOString(),
          last_activity_at: new Date(now - 30_000).toISOString(),
        }),
      ],
    })
    renderPage()
    const link = (await screen.findByRole('heading', { name: '活跃项目' })).closest('a') as HTMLElement
    expect(within(link).getByText(/30 seconds ago/)).toBeInTheDocument()
  })

  it('没有项目时显示引导，点按钮打开新建对话框', async () => {
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
    expect(await screen.findByRole('heading', { name: '恢复了' })).toBeInTheDocument()
  })
})

describe('新建项目', () => {
  const checkPath = 'POST /api/v1/boards/check-project-path'
  const open = async () => {
    const api = installFakeApi({
      'GET /api/v1/boards': [makeBoard({ id: 1, name: 'Existing' })],
      'GET /api/v1/templates': [
        { id: 1, name: 'dev', builtin: true },
        { id: 2, name: 'Team', builtin: false },
      ],
    })
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

  it('编号前缀不合法时提示并且不发请求', async () => {
    const { api, dialog } = await open()
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'X')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.slug)), '1bad')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    const alerts = await within(dialog).findAllByRole('alert')
    expect(alerts.map((a) => a.textContent)).toEqual([en.boards.create.errors.slugInvalid])
    expect(api.count('POST /api/v1/boards')).toBe(0)
  })

  it('用一个选择框列出模板，默认选中研发模板，不重复显示内置项', async () => {
    const { dialog } = await open()
    expect(within(dialog).getByRole('textbox', { name: /Working directory/ })).toHaveValue('')
    const select = within(dialog).getByRole('combobox', { name: en.boards.create.template })
    expect(select).toHaveValue('dev')
    expect(await within(select).findByRole('option', { name: 'Team' })).toBeInTheDocument()
    expect(within(select).getAllByRole('option').map(({ textContent }) => textContent)).toEqual([
      en.boards.create.templates.dev, en.boards.create.templates.empty, 'Team',
    ])
  })

  it('工作目录失焦时向服务端校验，成功才将整理后的路径随创建请求提交', async () => {
    const { api, dialog } = await open()
    api.set(checkPath, {})
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, '  /work/project  ')
    expect(api.count(checkPath)).toBe(0)
    await userEvent.tab()
    await waitFor(() => expect(api.count(checkPath)).toBe(1))
    expect(api.calls.find((c) => c.path.endsWith('/check-project-path'))?.body).toEqual({ project_path: '/work/project' })
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    await waitFor(() => expect(submit).toBeEnabled())
    await userEvent.click(submit)
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.path === '/api/v1/boards' && c.method === 'POST')?.body).toMatchObject({ project_path: '/work/project' })
  })

  it('目录不存在时显示字段错误，拦截提交；清空后仍可创建', async () => {
    const { api, dialog } = await open()
    api.set(checkPath, () => ({ status: 400, body: { reason: 'ErrProjectPathNotFound' } }))
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, '/work/missing')
    await userEvent.tab()
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('This directory does not exist.')
    expect(input).toHaveAttribute('aria-invalid', 'true')
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    expect(submit).toBeDisabled()
    fireEvent.submit(input.closest('form')!)
    expect(api.count('POST /api/v1/boards')).toBe(0)
    await userEvent.clear(input)
    await userEvent.tab()
    expect(submit).toBeEnabled()
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
    await userEvent.click(submit)
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.path === '/api/v1/boards' && c.method === 'POST')?.body).not.toHaveProperty('project_path')
  })

  it('未失焦直接回车也需要校验，网络失败禁止创建并可重试', async () => {
    const { api, dialog } = await open()
    api.set(checkPath, () => { throw new Error('network down') })
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, '/work/project{Enter}')
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Could not verify this directory. Please try again.')
    expect(api.count(checkPath)).toBe(1)
    expect(api.count('POST /api/v1/boards')).toBe(0)
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    expect(submit).toBeDisabled()
    api.set(checkPath, {})
    await userEvent.click(within(dialog).getByRole('button', { name: en.common.retry }))
    await waitFor(() => expect(submit).toBeEnabled())
    await userEvent.click(submit)
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
  })

  it('目录格式不合法时阻止提交，改成有效路径后可直接回车校验并创建', async () => {
    const { api, dialog } = await open()
    api.set(checkPath, () => ({ status: 400, body: { reason: 'ErrInvalidInput' } }))
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, 'relative/path')
    await userEvent.tab()
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Enter a valid absolute directory path on the server.')
    expect(within(dialog).getByRole('button', { name: en.boards.create.submit })).toBeDisabled()
    api.set(checkPath, {})
    await userEvent.clear(input)
    await userEvent.type(input, '/work/project{Enter}')
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.filter((c) => c.method === 'POST').map((c) => c.path)).toEqual([
      '/api/v1/boards/check-project-path', '/api/v1/boards/check-project-path', '/api/v1/boards',
    ])
  })

  it.each([false, true])('检查中禁止创建；切换路径后旧请求的成功=%s 不能覆盖新结果', async (oldValid) => {
    const { api, dialog } = await open()
    api.set(checkPath, oldValid ? () => ({ status: 400, body: { reason: 'ErrProjectPathNotFound' } }) : {})
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    const fallback = globalThis.fetch
    let resolveOld!: (response: Response) => void
    const oldResponse = new Promise<Response>((resolve) => { resolveOld = resolve })
    let oldSignal: AbortSignal | null | undefined
    vi.stubGlobal('fetch', vi.fn((url: RequestInfo | URL, init?: RequestInit) => {
      if (String(url).endsWith('/check-project-path') && init?.body === JSON.stringify({ project_path: '/old' })) {
        oldSignal = init.signal
        return oldResponse
      }
      return fallback(url, init)
    }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, '/old')
    await userEvent.tab()
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    expect(submit).toBeDisabled()
    fireEvent.submit(input.closest('form')!)
    expect(api.count('POST /api/v1/boards')).toBe(0)
    await userEvent.clear(input)
    await userEvent.type(input, '/new')
    expect(oldSignal?.aborted).toBe(true)
    await userEvent.tab()
    await waitFor(() => expect(api.count(checkPath)).toBe(1))
    if (oldValid) {
      expect(await within(dialog).findByRole('alert')).toHaveTextContent('This directory does not exist.')
    } else {
      await waitFor(() => expect(submit).toBeEnabled())
    }
    await act(async () => {
      resolveOld(oldValid ? new Response('{}') : new Response(JSON.stringify({ reason: 'ErrProjectPathNotFound' }), { status: 400 }))
      await oldResponse
    })
    if (oldValid) {
      expect(submit).toBeDisabled()
      expect(within(dialog).getByRole('alert')).toHaveTextContent('This directory does not exist.')
      fireEvent.submit(input.closest('form')!)
      expect(api.count('POST /api/v1/boards')).toBe(0)
      return
    }
    expect(submit).toBeEnabled()
    expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument()
    await userEvent.click(submit)
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.path === '/api/v1/boards' && c.method === 'POST')?.body).toMatchObject({ project_path: '/new' })
  })

  it('桌面客户端未填写工作目录时禁止创建，失焦提示必填', async () => {
    vi.stubGlobal('slateDesktopServer', { port: 49152, openPath: '/open' })
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    expect(submit).toBeDisabled()
    fireEvent.submit(within(dialog).getByLabelText(en.boards.create.name).closest('form')!)
    expect(api.count('POST /api/v1/boards')).toBe(0)
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.click(input)
    await userEvent.tab()
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(en.boards.projectPath.required)
    expect(submit).toBeDisabled()
  })

  it('预校验通过但保存时目录已消失，仍在目录字段显示错误', async () => {
    const { api, dialog } = await open()
    api.set(checkPath, {})
    api.set('POST /api/v1/boards', () => ({ status: 400, body: { reason: 'ErrProjectPathNotFound' } }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'Project')
    const input = within(dialog).getByRole('textbox', { name: /Working directory/ })
    await userEvent.type(input, '/work/vanished')
    await userEvent.tab()
    const submit = within(dialog).getByRole('button', { name: en.boards.create.submit })
    await waitFor(() => expect(submit).toBeEnabled())
    await userEvent.click(submit)
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('This directory does not exist.')
    expect(input).toHaveValue('/work/vanished')
    expect(submit).toBeDisabled()
  })

  it('选自定义模板后提交模板 ID', async () => {
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', makeSnapshot({ id: 9 }))
    const option = await within(dialog).findByRole('option', { name: 'Team' })
    await userEvent.selectOptions(within(dialog).getByRole('combobox', { name: en.boards.create.template }), option)
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), 'From Team')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    await waitFor(() => expect(api.count('POST /api/v1/boards')).toBe(1))
    expect(api.calls.find((c) => c.method === 'POST')?.body).toEqual({ name: 'From Team', project_template_id: 2 })
  })

  it('提交：编号前缀自动大写，名称去掉首尾空白，选中空白模板，带上当前界面语言，成功后跳到新项目', async () => {
    const { api, dialog } = await open()
    api.set('POST /api/v1/boards', makeSnapshot({ id: 7 }))
    await userEvent.type(within(dialog).getByLabelText(en.boards.create.name), '  My App  ')
    await userEvent.type(within(dialog).getByLabelText(new RegExp(en.boards.create.slug)), 'myapp')
    await userEvent.selectOptions(within(dialog).getByRole('combobox', { name: en.boards.create.template }), 'empty')
    await userEvent.click(within(dialog).getByRole('button', { name: en.boards.create.submit }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/b/7'))
    // 创建成功后项目列表缓存失效，切换菜单里能看到新项目
    await waitFor(() => expect(api.count('GET /api/v1/boards')).toBe(2))
    const post = api.calls.find((c) => c.method === 'POST')
    expect(post?.body).toEqual({ name: 'My App', slug: 'MYAPP', template: 'empty', locale: 'en' })
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
