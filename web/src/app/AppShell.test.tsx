import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import i18n from '@/i18n'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { installFakeApi } from '@/test/fakeApi'
import { makeBoard } from '@/test/factories'
import { THEME_KEY } from '@/lib/theme-context'
import { renderApp } from '@/test/renderApp'
import { AppRoutes } from './AppRoutes'

// 显示当前地址，用来断言跳转
function LocationProbe() {
  const { pathname, search } = useLocation()
  return (
    <output data-testid="location">
      {pathname}
      {search}
    </output>
  )
}

const connectStub = {
  base_url: 'http://127.0.0.1:7070',
  mcp_url: 'http://127.0.0.1:7070/mcp',
  install_url: 'http://127.0.0.1:7070/install.md',
  skill_url: 'http://127.0.0.1:7070/skills/slate-kanban/SKILL.md',
  loop_prompt: 'loop',
  watch_notes: [],
  hosts: [
    {
      id: 'cursor',
      name: 'Cursor',
      mcp: [{ kind: 'worker', title: '工作', lang: 'sh', code: 'mcp' }],
      skill: [],
      watch: [],
    },
  ],
}

const boards = [makeBoard({ id: 1, name: 'Slate', slug: 'SLATE' }), makeBoard({ id: 2, name: '官网', slug: 'WEB' })]

// 渲染完整应用，假服务端提供看板列表；auth 是访问口令的状态，默认没有启用口令
function setup(route = '/', auth = { required: false, authenticated: true }) {
  const api = installFakeApi({
    'GET /api/v1/boards': boards,
    'GET /api/v1/auth/status': auth,
    'GET /api/v1/connect': connectStub,
  })
  renderApp(
    <>
      <AppRoutes />
      <LocationProbe />
    </>,
    { route },
  )
  return api
}

async function openSettings() {
  await userEvent.click(await screen.findByRole('button', { name: en.nav.settings }))
  return within(await screen.findByRole('dialog', { name: en.nav.settings }))
}

describe('AppShell', () => {
  describe('桌面窗口拖动', () => {
    afterEach(() => {
      document.documentElement.removeAttribute('data-slate-mac')
      vi.unstubAllGlobals()
    })

    it('macOS 桌面端按住顶栏空白处可以拖动窗口，正文和右键不会拖动', async () => {
      document.documentElement.setAttribute('data-slate-mac', '')
      const invoke = vi.fn()
      vi.stubGlobal('_wails', { invoke })
      setup()
      const header = await screen.findByRole('banner')
      const settings = within(header).getByRole('button', { name: en.nav.settings })
      const main = screen.getByRole('main')

      // 一次普通点击不拖动，按住并移动才将手势交给原生窗口。
      fireEvent.mouseDown(header, { button: 0, buttons: 1 })
      fireEvent.mouseUp(header, { button: 0, buttons: 0 })
      expect(invoke).not.toHaveBeenCalled()

      fireEvent.mouseDown(header, { button: 0, buttons: 1 })
      fireEvent.blur(window)
      fireEvent.mouseMove(header, { buttons: 1 })
      expect(invoke).not.toHaveBeenCalled()

      fireEvent.mouseDown(header, { button: 0, buttons: 1 })
      expect(invoke).not.toHaveBeenCalled()
      fireEvent.mouseMove(main, { buttons: 1 })
      fireEvent.mouseMove(main, { buttons: 1 })
      fireEvent.mouseUp(main, { button: 0 })
      expect(invoke.mock.calls).toEqual([['wails:drag']])

      fireEvent.mouseDown(settings.parentElement!, { button: 0, buttons: 1 })
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      fireEvent.mouseDown(header, { button: 2, buttons: 2 })
      fireEvent.mouseMove(header, { buttons: 2 })
      fireEvent.mouseUp(header, { button: 2 })
      fireEvent.mouseDown(main, { button: 0, buttons: 1 })
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      expect(invoke.mock.calls).toEqual([['wails:drag'], ['wails:drag']])
    })

    it('macOS 桌面端顶栏的下拉框、链接、设置及弹窗仍能正常操作', async () => {
      document.documentElement.setAttribute('data-slate-mac', '')
      const invoke = vi.fn()
      vi.stubGlobal('_wails', { invoke })
      const user = userEvent.setup()
      setup()
      const header = await screen.findByRole('banner')
      const select = await within(header).findByRole('combobox', { name: en.nav.switchBoard })
      fireEvent.mouseDown(select, { button: 0, buttons: 1 })
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      await user.selectOptions(select, '1')
      await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/b/1'))
      await user.click(within(header).getByRole('link', { name: en.nav.agents }))
      await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/agents?project=slate'))
      await user.click(within(header).getByRole('button', { name: en.nav.settings }))
      const dialog = await screen.findByRole('dialog', { name: en.nav.settings })
      fireEvent.mouseDown(dialog, { button: 0, buttons: 1 })
      fireEvent.mouseMove(dialog, { buttons: 1 })
      fireEvent.mouseUp(dialog, { button: 0 })
      await user.keyboard('{Escape}')
      await user.click(within(header).getByRole('link', { name: en.app.name }))
      await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent(/^\/$/))
      expect(invoke).not.toHaveBeenCalled()
    })

    it('非 macOS 桌面端即使存在原生消息桥也不处理顶栏拖动', async () => {
      const invoke = vi.fn()
      vi.stubGlobal('_wails', { invoke })
      setup()
      const header = await screen.findByRole('banner')
      expect(fireEvent.mouseDown(header, { button: 0, buttons: 1 })).toBe(true)
      fireEvent.mouseMove(header, { buttons: 1 })
      fireEvent.mouseUp(header, { button: 0 })
      expect(invoke).not.toHaveBeenCalled()
    })

    it('原生消息桥尚未就绪时不拦截顶栏事件', async () => {
      document.documentElement.setAttribute('data-slate-mac', '')
      vi.stubGlobal('_wails', undefined)
      setup()
      expect(fireEvent.mouseDown(await screen.findByRole('banner'), { button: 0 })).toBe(true)
    })
  })

  it('有"跳到正文"链接，正文区域可以被它聚焦', async () => {
    setup()
    expect(await screen.findByRole('link', { name: en.nav.skipToContent })).toHaveAttribute('href', '#main')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main')
  })

  it('启用了访问口令且已登录时，设置里有退出按钮', async () => {
    setup('/', { required: true, authenticated: true })
    expect(screen.queryByRole('button', { name: en.auth.logout })).toBeNull()
    expect(await (await openSettings()).findByRole('button', { name: en.auth.logout })).toBeInTheDocument()
  })

  it('没有启用访问口令时，设置里没有退出按钮', async () => {
    const api = setup()
    const dialog = await openSettings()
    await waitFor(() => expect(api.count('GET /api/v1/auth/status')).toBe(1))
    expect(dialog.queryByRole('button', { name: en.auth.logout })).toBeNull()
  })

  it('切换语言：界面文字、<html lang>、i18n 当前语言一起变', async () => {
    setup()
    const select = (await openSettings()).getByRole('combobox', { name: en.language.label })
    await userEvent.selectOptions(select, 'ja')
    expect(await screen.findByRole('dialog', { name: ja.nav.settings })).toBeInTheDocument()
    await userEvent.keyboard('{Escape}')
    expect(await screen.findByRole('heading', { name: ja.boards.title })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('ja')
    expect(i18n.language).toBe('ja')
  })

  it('语言下拉里四种语言都用自己的文字显示', async () => {
    setup()
    const select = (await openSettings()).getByRole('combobox', { name: en.language.label })
    const options = within(select).getAllByRole('option')
    expect(options.map((o) => o.textContent)).toEqual(['简体中文', '繁體中文', 'English', '日本語'])
  })

  it('切换到深色主题：<html> 带上 dark 类', async () => {
    localStorage.setItem(THEME_KEY, 'light')
    setup()
    const themeSelect = (await openSettings()).getByRole('combobox', { name: en.theme.label })
    await userEvent.selectOptions(themeSelect, 'dark')
    await waitFor(() => expect(document.documentElement).toHaveClass('dark'))
  })

  it('看板切换：显示当前看板名，可以跳到别的看板或回到列表', async () => {
    setup('/b/2')
    const select = await screen.findByRole('combobox', { name: en.nav.switchBoard })
    expect(select).toHaveDisplayValue('官网')
    await userEvent.selectOptions(select, '1')
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/b/1'))
    await userEvent.selectOptions(select, en.nav.allBoards)
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent(/^\/$/))
  })

  it('不在看板页时，切换框选中「所有看板」', async () => {
    setup('/')
    const select = await screen.findByRole('combobox', { name: en.nav.switchBoard })
    expect(select).toHaveDisplayValue(en.nav.allBoards)
  })

  it('在 agent 页面时，agent 链接标为当前页', async () => {
    setup('/agents')
    const link = await screen.findByRole('link', { name: en.nav.agents })
    expect(link).toHaveAttribute('aria-current', 'page')
  })

  it('当前接入页链接保留 URL 绑定的看板', async () => {
    setup('/agents?project=slate')
    const link = await screen.findByRole('link', { name: en.nav.agents })
    expect(link).toHaveAttribute('href', '/agents?project=slate')
  })

  it('在看板页时，agent 链接带上当前看板的 project 参数', async () => {
    setup('/b/1')
    await screen.findByRole('combobox', { name: en.nav.switchBoard })
    const link = await screen.findByRole('link', { name: en.nav.agents })
    await waitFor(() => expect(link).toHaveAttribute('href', '/agents?project=slate'))
  })

  it('在首页时，agent 链接不带 project 参数', async () => {
    setup('/')
    const link = await screen.findByRole('link', { name: en.nav.agents })
    expect(link).toHaveAttribute('href', '/agents')
  })

  it('从看板页点 agent 进入带 project 且锁定看板', async () => {
    setup('/b/1')
    const link = await screen.findByRole('link', { name: en.nav.agents })
    await userEvent.click(link)
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/agents?project=slate'))
    const select = await screen.findByRole('combobox', { name: en.agents.watch.board })
    expect(select).toBeDisabled()
    await waitFor(() => expect(select).toHaveValue('1'))
  })

  it('地址不存在时显示 404 并能回到列表', async () => {
    setup('/nowhere')
    expect(await screen.findByRole('heading', { name: en.notFound.title })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: en.notFound.home }))
    expect(await screen.findByRole('heading', { name: en.boards.title })).toBeInTheDocument()
  })
})
