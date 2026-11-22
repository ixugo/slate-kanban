// 服务端自动重启后，等新版本起来就刷新页面。

import { useEffect, useState } from 'react'
import { getUpdateInfo } from '@/lib/api/endpoints'

/** 两次探测之间的间隔（毫秒）：重启通常一两秒就好 */
const POLL_INTERVAL_MS = 1000
/** 最多探测的次数：超过这么久还没起来，就让用户手动处理 */
const MAX_POLLS = 60

/** 去掉版本号开头的 v，v1.2.3 和 1.2.3 视为同一个版本 */
function bare(version: string): string {
  return version.replace(/^v/, '')
}

/**
 * 服务端要重启时（enabled 为真），每秒问一次当前运行的版本；
 * 等它变成刚更新到的版本，就刷新页面。重启期间连不上是正常的，下一轮再试。
 * 返回值为真表示等了太久还没起来，调用方应提示用户手动重启。
 */
export function useReloadAfterRestart(enabled: boolean, version: string): boolean {
  const [timedOut, setTimedOut] = useState(false)
  useEffect(() => {
    if (!enabled) return
    let polls = 0
    let stopped = false
    const timer = setInterval(() => {
      polls += 1
      getUpdateInfo()
        .then((info) => {
          if (stopped || bare(info.current) !== bare(version)) return
          stopped = true
          window.location.reload()
        })
        .catch(() => undefined)
      if (polls >= MAX_POLLS && !stopped) {
        stopped = true
        setTimedOut(true)
      }
    }, POLL_INTERVAL_MS)
    return () => {
      stopped = true
      clearInterval(timer)
    }
  }, [enabled, version])
  return timedOut
}
