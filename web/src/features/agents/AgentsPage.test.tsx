import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import AgentsPage from '@/features/agents/AgentsPage'
import type { ConnectGuide } from '@/lib/api/types'
import { installFakeApi } from '@/test/fakeApi'
import { renderApp } from '@/test/renderApp'

afterEach(() => vi.unstubAllGlobals())

const guide: ConnectGuide = {
  base_url: 'http://127.0.0.1:7070',
  mcp_url: 'http://127.0.0.1:7070/mcp',
  install_url: 'http://127.0.0.1:7070/install.md',
  skill_url: 'http://127.0.0.1:7070/skills/slate-kanban/SKILL.md',
  loop_prompt: '守候看板「<看板名>」',
  watch_notes: [],
  hosts: [
    {
      id: 'cursor',
      name: 'Cursor',
      mcp: [
        { kind: 'worker', title: '工作', lang: 'sh', code: 'cursor mcp --name=test' },
        { kind: 'manager', title: '管理', lang: 'sh', code: 'cursor mcp --role=manager' },
      ],
      skill: [{ kind: 'skill', title: 'skill', lang: 'sh', code: 'ln -s skill' }],
      watch: [{ kind: 'loop', title: '定时', lang: 'sh', code: '/loop 5m 一轮' }],
    },
  ],
}

describe('AgentsPage', () => {
  it('渲染片段并按 kind 显示标题', async () => {
    installFakeApi({ 'GET /api/v1/connect': guide })
    renderApp(<AgentsPage />, { route: '/agents' })
    expect(await screen.findByText(en.agents.title)).toBeInTheDocument()
    expect(screen.getByText(en.agents.snippets.worker)).toBeInTheDocument()
    expect(screen.getByText('cursor mcp --name=test')).toBeInTheDocument()
    expect(screen.getByText(en.agents.snippets.skill)).toBeInTheDocument()
  })

  it('先讲清持续监听的原理，skill 标明可选，命令片段收在手动安装折叠里', async () => {
    installFakeApi({ 'GET /api/v1/connect': guide })
    renderApp(<AgentsPage />, { route: '/agents' })
    expect(await screen.findByText(en.agents.watch.how.push)).toBeInTheDocument()
    expect(screen.getByText(en.agents.watch.how.connect)).toBeInTheDocument()
    expect(screen.getByText(en.agents.watch.how.loop)).toBeInTheDocument()
    const fold = screen.getByText(en.agents.advanced).closest('details')
    expect(fold).not.toHaveAttribute('open')
    expect(fold).toContainElement(screen.getByText(en.agents.skillOptional))
    expect(fold).toContainElement(screen.getByText('cursor mcp --name=test'))
  })

  it('长期挂着：选看板后提示词里的占位符换成看板名', async () => {
    installFakeApi({
      'GET /api/v1/connect': guide,
      'GET /api/v1/boards': [{ id: 1, name: '官网改版', engineers: [], tags: [] }],
    })
    renderApp(<AgentsPage />, { route: '/agents' })
    expect(await screen.findByText('守候看板「<看板名>」')).toBeInTheDocument()
    expect(screen.getByText(en.agents.snippets.loop)).toBeInTheDocument()
    await userEvent.selectOptions(await screen.findByRole('combobox', { name: en.agents.watch.board }), '官网改版')
    expect(screen.getByText('守候看板「官网改版」')).toBeInTheDocument()
  })

  it('复制失败时不抛错', async () => {
    installFakeApi({ 'GET /api/v1/connect': guide })
    const write = vi.fn().mockRejectedValue(new Error('denied'))
    Object.assign(navigator, { clipboard: { writeText: write } })
    renderApp(<AgentsPage />, { route: '/agents' })
    await screen.findByText(en.agents.title)
    const buttons = screen.getAllByRole('button', { name: en.common.copy })
    await userEvent.click(buttons[0])
    expect(write).toHaveBeenCalled()
  })

  it('加载失败可重试', async () => {
    const api = installFakeApi({
      'GET /api/v1/connect': () => ({ status: 500, body: { reason: 'ErrStore' } }),
    })
    renderApp(<AgentsPage />, { route: '/agents' })
    const retry = await screen.findByRole('button', { name: en.common.retry })
    api.set('GET /api/v1/connect', guide)
    await userEvent.click(retry)
    expect(await screen.findByText('cursor mcp --name=test')).toBeInTheDocument()
  })
})