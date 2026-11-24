import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { useTranslation } from '@/i18n/react'
import type { Board } from '@/lib/api/types'

declare global {
  interface Window {
    slateDesktopDispatch?: string
  }
}

interface DispatchConfig {
  board_id: number
  hosts: Record<string, string>
  enabled: boolean
}

interface DispatchState {
  config: DispatchConfig
  hosts: { id: string; name: string; available: boolean }[]
  status: Record<string, { state: string; card_id?: number; retry_at?: string }>
  engineers: { id: number; name: string; kind: string }[]
  template: boolean
  failure?: string
}

const copy = {
  'zh-CN': {
    title: '桌面自动派发', board: '选择研发看板', noHost: '暂不派发', unavailable: '未检测到',
    unavailableBoard: '此看板需要使用研发模板。', start: '启用并保存', stop: '停用', ready: '未启动',
    idle: '空闲', working: '执行中', unstable: '不稳定', blocked: '待核对', active: '已启用',
    error: '无法保存或读取桌面派发设置。请检查项目路径、选中的宿主和看板。',
    errorProject: '这个看板没有可用的本机项目路径。请在看板设置里填写已存在的 Git 项目目录后再保存。',
    errorHost: '没有可用的宿主。请为至少一位工程师选择本机已安装的终端工具。',
    errorSave: '无法写入桌面派发设置。请检查 Slate 配置目录的写入权限。',
    tech: '研发', review: '审查', choose: '先选择看板，再为工程师选择本机终端工具。',
  },
  'zh-TW': {
    title: '桌面自動派發', board: '選擇研發看板', noHost: '暫不派發', unavailable: '未偵測到',
    unavailableBoard: '此看板需要使用研發範本。', start: '啟用並儲存', stop: '停用', ready: '未啟動',
    idle: '閒置', working: '執行中', unstable: '不穩定', blocked: '待確認', active: '已啟用',
    error: '無法儲存或讀取桌面派發設定。請檢查專案路徑、所選工具和看板。',
    errorProject: '這個看板沒有可用的本機專案路徑。請在看板設定裡填寫已存在的 Git 專案目錄後再儲存。',
    errorHost: '沒有可用的工具。請為至少一位工程師選擇本機已安裝的終端工具。',
    errorSave: '無法寫入桌面派發設定。請檢查 Slate 設定目錄的寫入權限。',
    tech: '研發', review: '審查', choose: '先選擇看板，再為工程師選擇本機終端工具。',
  },
  ja: {
    title: 'デスクトップの自動割り当て', board: '開発ボードを選択', noHost: '割り当てなし', unavailable: '未検出',
    unavailableBoard: '開発テンプレートのボードが必要です。', start: '有効にして保存', stop: '無効化', ready: '未起動',
    idle: '待機中', working: '実行中', unstable: '不安定', blocked: '確認待ち', active: '有効',
    error: '設定を読み込み、または保存できません。プロジェクトパス、ツールとボードを確認してください。',
    errorProject: 'このボードにはローカルのプロジェクトパスがありません。ボード設定で既存の Git プロジェクトのディレクトリを入力してから保存してください。',
    errorHost: '利用できるツールがありません。少なくとも 1 人の担当者に、インストール済みのローカルツールを選択してください。',
    errorSave: 'デスクトップの割り当て設定を書き込めません。Slate の設定ディレクトリの書き込み権限を確認してください。',
    tech: '開発', review: 'レビュー', choose: 'ボードを選び、担当者ごとにローカルのツールを指定します。',
  },
  en: {
    title: 'Desktop dispatch', board: 'Select a development board', noHost: 'Skip this engineer', unavailable: 'Not found',
    unavailableBoard: 'This board must use the development template.', start: 'Enable and save', stop: 'Disable', ready: 'Disabled',
    idle: 'Idle', working: 'Working', unstable: 'Unstable', blocked: 'Needs attention', active: 'Enabled',
    error: 'Could not load or save dispatch settings. Check the project path, selected tools, and board.',
    errorProject: 'This board has no usable local project path. Set an existing Git project directory in the board settings, then save again.',
    errorHost: 'No usable tool is selected. Pick an installed local terminal tool for at least one engineer.',
    errorSave: 'Could not write the dispatch settings. Check write access to the Slate configuration directory.',
    tech: 'Development', review: 'Review', choose: 'Choose a board and assign a local terminal tool to each engineer.',
  },
}

export function DesktopDispatch({ bridge, boards = [] }: { bridge: string; boards: Board[] | undefined }) {
  const { i18n } = useTranslation()
  const language = i18n.language as keyof typeof copy
  const label = copy[language] ?? copy.en
  const [boardId, setBoardId] = useState(0)
  const [state, setState] = useState<DispatchState>()
  const [hosts, setHosts] = useState<Record<string, string>>({})
  const [loadedBoard, setLoadedBoard] = useState(-1)
  // 失败原因：空串表示没有失败；服务端给出的原因标识用于显示具体提示，其余按通用提示处理。
  const [problem, setProblem] = useState('')
  const [pending, setPending] = useState(false)

  useEffect(() => {
    const controller = new AbortController()
    const refresh = async () => {
      try {
        const res = await fetch(`${bridge}?board_id=${boardId}`, {
          headers: { 'X-Slate-Desktop': 'dispatch' }, signal: controller.signal,
        })
        if (!res.ok) throw new Error()
        const next: DispatchState = await res.json()
        if (controller.signal.aborted) return
        if (boardId === 0 && loadedBoard < 0 && next.config.board_id) setBoardId(next.config.board_id)
        setState(next)
        if (loadedBoard !== boardId) {
          setHosts(next.config.board_id === boardId ? (next.config.hosts ?? {}) : {})
          setLoadedBoard(boardId)
        }
      } catch {
        if (!controller.signal.aborted) setProblem('generic')
      }
    }
    void refresh()
    const timer = window.setInterval(() => void refresh(), 4000)
    return () => {
      controller.abort()
      window.clearInterval(timer)
    }
  }, [bridge, boardId, loadedBoard])

  const save = async (enabled: boolean) => {
    setPending(true)
    setProblem('')
    try {
      const target = enabled ? boardId : state?.config.board_id
      const selected = enabled ? hosts : state?.config.hosts
      const res = await fetch(bridge, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', 'X-Slate-Desktop': 'dispatch' },
        body: JSON.stringify({ board_id: target, hosts: selected, enabled }),
      })
      if (!res.ok) {
        const body: { code?: string } = await res.json().catch(() => ({}))
        setProblem(body.code || 'generic')
        return
      }
      const next: DispatchState = await res.json()
      setState(next)
      if (target !== boardId) setBoardId(next.config.board_id)
    } catch {
      setProblem('generic')
    } finally {
      setPending(false)
    }
  }

  const config = state?.config
  const problemText = ({ project: label.errorProject, host: label.errorHost, save: label.errorSave, template: label.unavailableBoard } as Record<string, string>)[problem] ?? label.error
  const live = config?.enabled && !state?.failure
  const statusLabel = (state?: string) => {
    switch (state) {
      case 'working': return label.working
      case 'unstable': return label.unstable
      case 'blocked': return label.blocked
      default: return label.idle
    }
  }

  return (
    <section className="space-y-4 rounded-2xl bg-card p-5 shadow-card">
      <header className="flex items-center justify-between gap-3">
        <h2 className="text-body font-semibold tracking-tight">{label.title}</h2>
        <span className="text-ui text-muted-foreground">{live ? label.active : label.ready}</span>
      </header>
      <label className="grid gap-1 text-ui">
        {label.board}
        <select className="h-8 rounded-md border bg-background px-2" value={boardId} onChange={(e) => setBoardId(Number(e.target.value))}>
          <option value={0}>—</option>
          {boards.map((board) => <option key={board.id} value={board.id}>{board.name}</option>)}
        </select>
      </label>
      {!boardId && <p className="text-ui text-muted-foreground">{label.choose}</p>}
      {boardId > 0 && loadedBoard === boardId && state && !state.template && <p role="status" className="text-ui">{label.unavailableBoard}</p>}
      {boardId > 0 && loadedBoard === boardId && state?.template && (
        <div className="divide-y border-y">
          {state.engineers.map((engineer) => {
            const status = config?.board_id === boardId ? state.status[engineer.id] : undefined
            return (
              <div key={engineer.id} className="flex flex-wrap items-center justify-between gap-3 py-2.5 text-ui">
                <div>
                  <p className="font-medium">{engineer.name}</p>
                  <p className="text-muted-foreground">{engineer.kind === 'review' ? label.review : label.tech} · {statusLabel(status?.state)}
                    {status?.card_id ? ` #${status.card_id}` : ''}
                    {status?.retry_at ? ` · ${new Date(status.retry_at).toLocaleTimeString()}` : ''}
                  </p>
                </div>
                <select aria-label={engineer.name} className="h-8 rounded-md border bg-background px-2" value={hosts[engineer.id] ?? ''}
                  onChange={(e) => setHosts((previous) => {
                    const next = { ...previous }
                    if (e.target.value) next[engineer.id] = e.target.value
                    else delete next[engineer.id]
                    return next
                  })}
                >
                  <option value="">{label.noHost}</option>
                  {state.hosts.map((host) => <option key={host.id} value={host.id} disabled={!host.available}>{host.name}{host.available ? '' : ` · ${label.unavailable}`}</option>)}
                </select>
              </div>
            )
          })}
        </div>
      )}
      {(problem || state?.failure) && <p role="alert" className="text-ui text-destructive">{problem ? problemText : label.error}</p>}
      <div className="flex gap-3">
        <Button type="button" size="sm" disabled={pending || !boardId || loadedBoard !== boardId || !state?.template || !Object.keys(hosts).length} onClick={() => void save(true)}>{label.start}</Button>
        {config?.enabled && <Button type="button" variant="outline" size="sm" disabled={pending} onClick={() => void save(false)}>{label.stop}</Button>}
      </div>
    </section>
  )
}
