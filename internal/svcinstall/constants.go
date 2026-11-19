package svcinstall

// launchdLabel 是 macOS LaunchAgent 在系统里的唯一标识，launchctl 子命令里也用这个名字。
const launchdLabel = "dev.slate.kanban"

// launchdPlistName 是写到 ~/Library/LaunchAgents/ 下的 plist 文件名。
const launchdPlistName = "dev.slate.kanban.plist"

// systemdUnitName 是 Linux 用户级 systemd 单元文件名（放在 ~/.config/systemd/user/ 下）。
const systemdUnitName = "slate.service"

// stdoutLogName 是 launchd 托管进程的标准输出日志文件名（在配置目录的 logs/ 下）。
const stdoutLogName = "slate-service.stdout.log"

// stderrLogName 是 launchd 托管进程的标准错误日志文件名（在配置目录的 logs/ 下）。
const stderrLogName = "slate-service.stderr.log"

// plainPathChars 是路径里不需要加引号的字符；出现其它字符就整个路径加引号，宁可多加也不漏。
// systemd 的 ExecStart 参数和打印给用户粘贴的 shell 命令共用这一份。
const plainPathChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@+=:,./-"

// maxConfArgLen 是 -conf 路径的长度上限，防止异常长的输入写进服务文件。
const maxConfArgLen = 4096
