// TanStack Query 的缓存键集中放在这里：失效、乐观更新、实时刷新都靠同一套键对得上。

export const queryKeys = {
  /** 看板列表 */
  boards: ['boards'] as const,
  /** 一块看板的整板快照 */
  board: (boardId: number) => ['board', boardId] as const,
  /** 一张卡片的详情；不带编号表示全部卡片详情，用来整体失效 */
  card: (cardId?: number) => (cardId === undefined ? (['card'] as const) : (['card', cardId] as const)),
  /** 接入说明 */
  connect: ['connect'] as const,
}
