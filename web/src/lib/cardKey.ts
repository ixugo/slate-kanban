// 任务编号的显示：数据库和接口里存的是 SLATE-40 这样的 key，网页上只显示成 #40。

/** 把 SLATE-40 显示成 #40；不是「前缀-数字」的格式就原样返回，不会丢信息 */
export function displayKey(key: string): string {
  const match = /^.+-(\d+)$/.exec(key)
  return match ? `#${match[1]}` : key
}
