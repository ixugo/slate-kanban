// TanStack Query 的缓存键集中放在这里：失效、乐观更新、实时刷新都靠同一套键对得上。

export const queryKeys = {
  /** 访问口令的状态 */
  auth: ['auth'] as const,
  /** 看板列表 */
  boards: ['boards'] as const,
  /** 一块看板的整板快照 */
  board: (boardId: number) => ['board', boardId] as const,
  /** 一块看板的活动流，不同的筛选条件各存一份；挂在看板键下面，看板有更新时会跟着刷新 */
  activity: (boardId: number, filters: object) => ['board', boardId, 'activity', filters] as const,
  /** 一张卡片的事件（卡片详情里的「评论和活动」）；挂在看板键下面，看板有更新时会跟着刷新 */
  cardActivity: (boardId: number, cardId: number) => ['board', boardId, 'card-activity', cardId] as const,
  /** 一张卡片的详情；不带编号表示全部卡片详情，用来整体失效 */
  card: (cardId?: number) => (cardId === undefined ? (['card'] as const) : (['card', cardId] as const)),
  /** 服务端版本信息 */
  version: ['version'] as const,
  /** 检查更新的结果 */
  update: ['update'] as const,
  /** 接入说明 */
  connect: ['connect'] as const,
}
