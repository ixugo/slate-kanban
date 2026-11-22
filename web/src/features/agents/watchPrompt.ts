import type { Board } from '@/lib/api/types'

/** 提示词里的看板名占位符，和服务端 connectapi 的 boardPlaceholder 一致 */
const BOARD_PLACEHOLDER = '<看板名>'

// Codex 的命令把提示词放在单引号内，名字里的单引号需要按 shell 字面量转义。
export function bindWatchPrompt(prompt: string, board: Pick<Board, 'id' | 'name'>, shellQuoted = false): string {
  const name = shellQuoted ? board.name.replaceAll("'", "'\\''") : board.name
  // 只绑定守候提示中的工具调用，保留宿主命令里说明 project_path 的注释。
  return prompt
    .replace('调用 get_board', `调用 get_board({"board_id":${board.id}})`)
    .replaceAll(BOARD_PLACEHOLDER, () => name)
    .replaceAll('<看板编号>', String(board.id))
}
