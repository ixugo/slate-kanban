// 端到端测试共用的环境约定：测试服务监听哪个端口，浏览器访问哪个地址。

/** 测试服务默认监听的端口，避开了本机 7070 的真实服务 */
const DEFAULT_PORT = 17420

/**
 * 读取测试服务的端口：环境变量 E2E_PORT 没设就用默认值；
 * 设了就必须是 1024 到 65535 的整数，写错了立刻报错，而不是悄悄连到别的端口。
 */
export function readPort(raw: string | undefined = process.env.E2E_PORT): number {
  if (raw === undefined || raw === '') return DEFAULT_PORT
  const port = Number(raw)
  if (!/^\d{1,5}$/.test(raw) || port < 1024 || port > 65535) {
    throw new Error(`E2E_PORT 必须是 1024 到 65535 的整数，当前是 ${JSON.stringify(raw.slice(0, 20))}`)
  }
  return port
}

/** 测试服务的端口 */
export const PORT = readPort()

/** 测试服务的地址，只监听本机 */
export const BASE_URL = `http://127.0.0.1:${PORT}`
