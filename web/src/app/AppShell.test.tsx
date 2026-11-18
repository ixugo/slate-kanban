import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import i18n from '@/i18n'
import { en } from '@/i18n/locales/en'
import { ja } from '@/i18n/locales/ja'
import { installFakeApi } from '@/test/fakeApi'
import { makeBoard } from '@/test/factories'
import { renderApp } from '@/test/renderApp'
import { AppRoutes } from './AppRoutes'

// 显示当前地址，用来断言跳转
function LocationProbe() {
  return <output data-testid="location">{useLocation().pathname}</output>
}

const boards = [makeBoard({ id: 1, name: 'Slate', slug: 'SLATE' }), makeBoard({ id: 2, name: '官网', slug: 'WEB' })]

// 渲染完整应用，假服务端提供看板列表
function setup(route = '/') {
  const api = installFakeApi({ 'GET /api/v1/boards': boards })
  renderApp(
    <>
      <AppRoutes />
      <LocationProbe />
    </>,
    { route },
  )
  return api
}

describe('AppShell', () => {
  it('有"跳到正文"链接，正文区域可以被它聚焦', async () => {
    setup()
    expect(await screen.findByRole('link', { name: en.nav.skipToContent })).toHaveAttribute('href', '#main')
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main')
  })

  it('切换语言：界面文字、<html lang>、i18n 当前语言一起变', async () => {
    setup()
    await userEvent.click(await screen.findByRole('button', { name: en.language.label }))
    await userEvent.click(await screen.findByRole('menuitemradio', { name: '日本語' }))
    expect(await screen.findByRole('heading', { name: ja.boards.title })).toBeInTheDocument()
    expect(document.documentElement.lang).toBe('ja')
    expect(i18n.language).toBe('ja')
  })

  it('菜单里五种语言都用自己的文字显示', async () => {
    setup()
    await userEvent.click(await screen.findByRole('button', { name: en.language.label }))
    const items = await screen.findAllByRole('menuitemradio')
    expect(items.map((i) => i.textContent)).toEqual(['简体中文', '繁體中文', 'English', '日本語', '한국어'])
  })

  it('切换到深色主题：<html> 带上 dark 类', async () => {
    setup()
    await userEvent.click(await screen.findByRole('button', { name: en.theme.label }))
    await userEvent.click(await screen.findByRole('menuitemradio', { name: en.theme.dark }))
    await waitFor(() => expect(document.documentElement).toHaveClass('dark'))
  })

  it('看板切换：显示当前看板名，可以跳到别的看板或回到列表', async () => {
    setup('/b/2')
    const trigger = await screen.findByRole('button', { name: /官网/ })
    await userEvent.click(trigger)
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('menuitem', { name: /官网/ })).toBeInTheDocument()
    await userEvent.click(within(menu).getByRole('menuitem', { name: /Slate/ }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/b/1'))
    await userEvent.click(await screen.findByRole('button', { name: /Slate/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: en.nav.allBoards }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent(/^\/$/))
  })

  it('不在看板页时，切换按钮显示"看板"', async () => {
    setup('/')
    expect(await screen.findByRole('button', { name: en.nav.boards })).toBeInTheDocument()
  })

  it('在 agent 页面时，agent 链接标为当前页', async () => {
    setup('/agents')
    const link = await screen.findByRole('link', { name: en.nav.agents })
    expect(link).toHaveAttribute('aria-current', 'page')
  })

  it('地址不存在时显示 404 并能回到列表', async () => {
    setup('/nowhere')
    expect(await screen.findByRole('heading', { name: en.notFound.title })).toBeInTheDocument()
    await userEvent.click(screen.getByRole('link', { name: en.notFound.home }))
    expect(await screen.findByRole('heading', { name: en.boards.title })).toBeInTheDocument()
  })
})
