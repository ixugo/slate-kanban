// 全局准备：编译带前端的 Slate 程序，在临时目录里用全新的数据库启动它；全部用例跑完后停止并清理。
// 配置、数据库、日志和编译出的程序都放在临时目录里，不会碰 ~/.slate-kanban 里的真实数据，也不占本机 7070 端口。

import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import { closeSync, existsSync, mkdtempSync, openSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { BASE_URL, PORT } from './env'

/** 仓库根目录（web/ 的上一级），go build 要在这里执行 */
const REPO_ROOT = resolve(import.meta.dirname, '../..')

/** 前端构建产物的入口页；没有它，带前端的程序编译不过 */
const DIST_INDEX = join(REPO_ROOT, 'internal/web/spa/dist/index.html')

/** 等服务起来最多等多久，单位毫秒 */
const STARTUP_TIMEOUT_MS = 30_000

/** 停服务时先让它自己退出，等这么久还没退出就强杀，单位毫秒 */
const STOP_TIMEOUT_MS = 5_000

/** 出错时回显日志的最后多少行 */
const LOG_TAIL_LINES = 30

/** 等待若干毫秒 */
function sleep(ms: number): Promise<void> {
  return new Promise((done) => setTimeout(done, ms))
}

/** 测试端口上的服务是否已经能回应健康检查；连不上就说明还没起来（或根本没有服务） */
async function isUp(): Promise<boolean> {
  try {
    const res = await fetch(`${BASE_URL}/health`, { signal: AbortSignal.timeout(1000) })
    return res.ok
  } catch {
    return false
  }
}

/** 日志文件的最后几行，启动失败时拿来说明原因 */
function logTail(file: string): string {
  if (!existsSync(file)) return '（没有日志）'
  return readFileSync(file, 'utf8').trimEnd().split('\n').slice(-LOG_TAIL_LINES).join('\n')
}

/** 编译带前端的 Slate 程序到临时目录，返回程序路径。前端没构建过就直接提示，免得编译报一长串嵌入错误 */
function buildServer(dir: string): string {
  if (!existsSync(DIST_INDEX)) {
    throw new Error('找不到前端构建产物 internal/web/spa/dist，请先运行 npm run build（npm run e2e 会自动先构建）')
  }
  const bin = join(dir, process.platform === 'win32' ? 'slate.exe' : 'slate')
  console.log('编译 Slate（带前端）……')
  try {
    execFileSync('go', ['build', '-tags', 'embed_web', '-o', bin, '.'], {
      cwd: REPO_ROOT,
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '0' },
    })
  } catch (err) {
    throw new Error(`编译 Slate 失败：${err instanceof Error ? err.message : String(err)}`, { cause: err })
  }
  return bin
}

/** 在临时目录里写一份最小配置：只监听本机的测试端口，数据库和日志都放在临时目录里 */
function writeConfig(dir: string): void {
  const toml = `
[Server]
  [Server.HTTP]
    Host = '127.0.0.1'
    Port = ${PORT}
    Timeout = '30s'
    AllowedHosts = []
    Passcode = ''
    [Server.HTTP.PProf]
      Enabled = false
      AccessIps = []

[Data]
  [Data.Database]
    Dsn = ${JSON.stringify(join(dir, 'data.db'))}
    MaxIdleConns = 1
    MaxOpenConns = 1
    ConnMaxLifetime = '6h0m0s'
    SlowThreshold = '200ms'

[Log]
  Dir = ${JSON.stringify(join(dir, 'logs'))}
  Level = 'info'
  MaxAge = 1
  RotationTime = '8h0m0s'
  MaxSize = 50
  Compress = false
  MaxBackups = 1
`
  writeFileSync(join(dir, 'config.toml'), toml.trimStart())
}

/** 启动程序：工作目录和配置目录都是临时目录，标准输出和错误输出写进 server.log */
function startServer(bin: string, dir: string, logFile: string): ChildProcess {
  const fd = openSync(logFile, 'a')
  try {
    return spawn(bin, ['-conf', dir], { cwd: dir, stdio: ['ignore', fd, fd] })
  } finally {
    // 子进程已经拿到了自己的一份文件描述符，这里关掉自己的
    closeSync(fd)
  }
}

/** 等服务能回应健康检查；程序中途退出或超时都抛错，并带上日志末尾 */
async function waitUntilUp(child: ChildProcess, logFile: string): Promise<void> {
  const deadline = Date.now() + STARTUP_TIMEOUT_MS
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      throw new Error(`Slate 启动后退出了（退出码 ${child.exitCode}），日志末尾：\n${logTail(logFile)}`)
    }
    if (await isUp()) return
    await sleep(100)
  }
  throw new Error(`${STARTUP_TIMEOUT_MS / 1000} 秒内 Slate 没有就绪，日志末尾：\n${logTail(logFile)}`)
}

/** 停掉服务：先请它自己退出，超时再强杀；已经退出就什么都不做 */
async function stopServer(child: ChildProcess): Promise<void> {
  if (child.exitCode !== null) return
  const exited = new Promise<void>((done) => child.once('exit', () => done()))
  child.kill('SIGTERM')
  const timer = setTimeout(() => child.kill('SIGKILL'), STOP_TIMEOUT_MS)
  await exited
  clearTimeout(timer)
}

/** 删除本次运行创建的临时目录（里面只有这次运行自己生成的文件） */
function cleanup(dir: string): void {
  rmSync(dir, { recursive: true, force: true })
}

/** Playwright 在全部用例之前调用；返回的函数在全部用例之后调用，负责停服务和清理 */
export default async function globalSetup(): Promise<() => Promise<void>> {
  if (await isUp()) {
    throw new Error(`端口 ${PORT} 上已经有服务在运行。先停掉它，或者用环境变量 E2E_PORT 换一个端口`)
  }
  const dir = mkdtempSync(join(tmpdir(), 'slate-e2e-'))
  const logFile = join(dir, 'server.log')
  let child: ChildProcess | undefined
  try {
    const bin = buildServer(dir)
    writeConfig(dir)
    child = startServer(bin, dir, logFile)
    await waitUntilUp(child, logFile)
  } catch (err) {
    if (child) await stopServer(child)
    cleanup(dir)
    throw err
  }
  const server = child
  return async () => {
    await stopServer(server)
    cleanup(dir)
  }
}
