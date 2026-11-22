// 乐观更新的公共步骤：先把预测结果写进缓存让界面立刻变化，请求失败再整块还原。

import type { QueryClient, QueryKey } from '@tanstack/react-query'

/**
 * 取消该键上还在飞的查询（免得旧响应回来盖掉预测结果），记下旧值，再写入 patch 算出的新值。
 * 缓存里还没有数据时不写，返回 undefined。返回值交给 rollbackOptimistic 还原。
 */
export async function applyOptimistic<T>(
  queryClient: QueryClient,
  key: QueryKey,
  patch: (old: T) => T,
): Promise<T | undefined> {
  // 只取消这一个键：看板键下面还挂着活动流等别的查询，不该被连带中止
  await queryClient.cancelQueries({ queryKey: key, exact: true })
  const previous = queryClient.getQueryData<T>(key)
  if (previous !== undefined) queryClient.setQueryData<T>(key, patch(previous))
  return previous
}

/** 把 applyOptimistic 记下的旧值写回缓存；当时没有旧值就什么都不做 */
export function rollbackOptimistic<T>(queryClient: QueryClient, key: QueryKey, previous: T | undefined): void {
  if (previous !== undefined) queryClient.setQueryData<T>(key, previous)
}
