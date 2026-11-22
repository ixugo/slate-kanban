import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { en } from '@/i18n/locales/en'
import AgentsPage from '@/features/agents/AgentsPage'
import type { ConnectGuide } from '@/lib/api/types'
import { installFakeApi } from '@/test/fakeApi'
import { makeBoard } from '@/test/factories'
import { renderApp } from '@/test/renderApp'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const loopPrompt = '守候 Slate 看板「<看板名>」：调用 get_board 读取返回里的 watch_prompt，照它做。'
const sameName = "官网 $& '改版"
const eventWatchCommand = (host: string, name = host) =>
  `python "slate-watch.py" --url "http://127.0.0.1:7070/mcp" --board "<看板编号>" --name "${name}" --project "<项目目录>" --host ${host}`
const boundPrompt = (id: number) => `守候 Slate 看板「${sameName}」：调用 get_board({"board_id":${id}}) 读取返回里的 watch_prompt，照它做。`

const guide: ConnectGuide = {
  base_url: 'http://127.0.0.1:7070',
  mcp_url: 'http://127.0.0.1:7070/mcp',
  install_url: 'http://127.0.0.1:7070/install.md',
  skill_url: 'http://127.0.0.1:7070/skills/slate-kanban/SKILL.md',
  loop_prompt: loopPrompt,
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
      watch: [
        { kind: 'event-watch', title: '启动', lang: 'sh', code: eventWatchCommand('cursor') },
        { kind: 'loop', title: '定时', lang: 'sh', code: `/loop 5m ${loopPrompt}` },
      ],
    },
    {
      id: 'claude-code',
      name: 'Claude Code',
      mcp: [],
      skill: [],
      watch: [
        { kind: 'event-watch', title: '启动', lang: 'sh', code: eventWatchCommand('claude', 'claude-code') },
        { kind: 'watch', title: '前台', lang: 'sh', code: `claude\n/loop 5m ${loopPrompt}` },
      ],
    },
    {
      id: 'codex',
      name: 'Codex',
      mcp: [],
      skill: [],
      watch: [{ kind: 'event-watch', title: '启动', lang: 'sh', code: eventWatchCommand('codex') }],
    },
  ],
}

const boards = [
  makeBoard({ id: 1, name: sameName, slug: 'WEB' }),
  makeBoard({ id: 2, name: sameName, slug: 'SLATE' }),
]

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

  it('按宿主提供仅移除 Slate 工作和管理视图的说明及命令', async () => {
    const user = userEvent.setup()
    installFakeApi({ 'GET /api/v1/connect': guide })
    renderApp(<AgentsPage />, { route: '/agents' })
    await user.click(await screen.findByText(en.agents.advanced))

    const cursor = screen.getByText(en.agents.uninstallMcp)
    expect(cursor.closest('details')).not.toHaveAttribute('open')
    await user.click(cursor)
    expect(screen.getByText(en.agents.uninstallMcpCursor)).toBeVisible()

    for (const [host, command] of [['Claude Code', 'claude mcp remove --scope user'], ['Codex', 'codex mcp remove']]) {
      await user.click(screen.getByRole('tab', { name: host }))
      const summary = screen.getByText(en.agents.uninstallMcp)
      await user.click(summary)
      const details = summary.closest('details')!
      const section = within(details)
      expect(section.getByText(`${command} slate`)).toBeVisible()
      expect(section.getByText(`${command} slate-manager`)).toBeVisible()
      const [copyWorker, copyManager] = section.getAllByRole('button', { name: en.common.copy })
      await user.click(copyWorker)
      expect(await navigator.clipboard.readText()).toBe(`${command} slate`)
      await user.click(copyManager)
      expect(await navigator.clipboard.readText()).toBe(`${command} slate-manager`)
    }
  })

  it('无 project 参数时，可以按 value 选择两个同名看板', async () => {
    installFakeApi({ 'GET /api/v1/connect': guide, 'GET /api/v1/boards': boards })
    renderApp(<AgentsPage />, { route: '/agents' })
    const select = await screen.findByRole('combobox', { name: en.agents.watch.board })
    expect(screen.queryByRole('heading', { name: en.agents.watch.loopTitle })).not.toBeInTheDocument()
    await userEvent.click(screen.getByText(en.agents.advanced))
    expect(screen.getByText(`/loop 5m ${loopPrompt}`)).toBeInTheDocument()

    await userEvent.selectOptions(select, "1")
    expect(screen.getByText(boundPrompt(1))).toBeInTheDocument()
    expect(screen.getByText(`/loop 5m ${boundPrompt(1)}`)).toBeInTheDocument()
    await userEvent.selectOptions(select, "2")
    expect(screen.getByText(boundPrompt(2))).toBeInTheDocument()
    expect(screen.getByText(`/loop 5m ${boundPrompt(2)}`)).toBeInTheDocument()
    expect(screen.queryByText(boundPrompt(1))).not.toBeInTheDocument()
  })

  it.each(boards)('project=$slug 固定同名看板之一的各宿主命令', async ({ id, slug }) => {
    installFakeApi({ 'GET /api/v1/connect': guide, 'GET /api/v1/boards': boards })
    renderApp(<AgentsPage />, { route: `/agents?project=${slug.toLowerCase()}` })

    const prompt = boundPrompt(id)
    expect(await screen.findByText(prompt)).toBeInTheDocument()
    const select = screen.getByRole('combobox', { name: en.agents.watch.board })
    expect(select).toBeDisabled()
    expect(select).toHaveValue(String(id))

    await userEvent.click(screen.getByText(en.agents.advanced))
    expect(screen.getByText(`/loop 5m ${prompt}`)).toBeInTheDocument()
    expect(screen.getByText('cursor mcp --name=test')).toBeInTheDocument()
    expect(screen.getByText(eventWatchCommand('cursor').replace('<看板编号>', String(id)))).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Claude Code' }))
    expect(screen.getByText(`claude /loop 5m ${prompt}`)).toBeInTheDocument()
    expect(screen.getByText(eventWatchCommand('claude', 'claude-code').replace('<看板编号>', String(id)))).toBeInTheDocument()
    await userEvent.click(screen.getByRole('tab', { name: 'Codex' }))
    expect(screen.getByText(eventWatchCommand('codex').replace('<看板编号>', String(id)))).toBeInTheDocument()
  })

  it('project 为空串时当作未指定，仍可选择看板', async () => {
    installFakeApi({ 'GET /api/v1/connect': guide, 'GET /api/v1/boards': boards })
    renderApp(<AgentsPage />, { route: '/agents?project=' })
    expect(await screen.findByRole('combobox', { name: en.agents.watch.board })).toBeInTheDocument()
  })

  it.each(['missing'])('project=%s 无法对应看板时提示错误，但下拉框不锁死、仍可选择', async (project) => {
    installFakeApi({ 'GET /api/v1/connect': guide, 'GET /api/v1/boards': boards })
    renderApp(<AgentsPage />, { route: `/agents?project=${project}` })
    expect(await screen.findByRole('alert')).toHaveTextContent('project')
    const select = screen.getByRole('combobox', { name: en.agents.watch.board })
    expect(select).toBeEnabled()
    expect(screen.queryByRole('heading', { name: en.agents.watch.loopTitle })).not.toBeInTheDocument()
    await userEvent.selectOptions(select, "1")
    expect(screen.getByText(boundPrompt(1))).toBeInTheDocument()
  })

  it('复制失败时不抛错', async () => {
    const user = userEvent.setup()
    installFakeApi({ 'GET /api/v1/connect': guide })
    const write = vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('denied'))
    renderApp(<AgentsPage />, { route: '/agents' })
    await screen.findByText(en.agents.title)
    const buttons = screen.getAllByRole('button', { name: en.common.copy })
    await user.click(buttons[0])
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
