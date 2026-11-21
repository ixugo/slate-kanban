# 系统服务注册（internal/svcmgr）

## 需求目标

`slate -d` 把 Slate 注册为系统服务并启动，`slate -stop` 停止并删除服务。
底层用 [kardianos/service](https://github.com/kardianos/service)，覆盖三个平台：

| 平台 | 服务形式 | 位置 | 何时启动 | 权限 |
|------|----------|------|----------|------|
| Windows | Windows 服务（SCM） | 服务管理器，服务名 `slate` | 开机自动 | 需管理员身份打开终端 |
| Linux | 系统级 systemd 单元，`User=` 为 sudo 发起者 | `/etc/systemd/system/slate.service` | 开机即启，不依赖登录 | 非 root 时自动 `sudo` 重新执行 |
| macOS | 用户级 LaunchAgent，Label `dev.slate.kanban` | `~/Library/LaunchAgents/` | 登录后启动 | 不需要 sudo |

配置目录默认 `~/.slate-kanban`（`conf.DefaultDirName`），`-d` 时把绝对路径的 `-conf <目录>` 写进服务参数，所以服务方式和直接运行用同一份数据。Linux 用 sudo 时主目录取 `SUDO_USER`，不会落到 `/root`。

## 行为

- `-d`：先停止并删除旧注册（没有就跳过），再 Install + Start，所以重复执行总是「用当前程序和配置运行」。
- `-stop`：已注册则停止（运行中才停）并删除；没注册直接成功，提示「没有已注册的 Slate 服务」。
- `-d` 与 `-stop` 互斥；`-conf` 校验：非空、不超过 4096 字节、不含控制字符。
- macOS 日志写到 `<配置目录>/logs/dev.slate.kanban.{out,err}.log`，`-d` 时先建 `logs/`。
- Windows 由服务管理器拉起时（`service.Interactive()` 为 false），`main.go` 走 `svcmgr.RunAsService`：服务停止时关闭 `stop` 通道，`app.Run` 优雅退出，最多等 30 秒；主流程自己退出则进程以 1 退出。Linux、macOS 仍直接运行并处理信号。
- 发布包里带 `start.sh` / `stop.sh`（Linux、macOS）和 `start.bat` / `stop.bat`（Windows），分别等同 `slate -d` / `slate -stop`。

## 已知限制

- kardianos/service 的 systemd 单元里 `RestartSec=120`（崩溃后 2 分钟才重启），`Restart` 已设为 `on-failure`。
- systemd 的 Uninstall 只 disable 不 stop，所以 `removeService` 在服务运行中时先 Stop。
- 真机验证：macOS 的 `-d`、重复 `-d`、`-stop`、重复 `-stop`（用临时配置目录和 17070 端口）。Windows、Linux 只做了 `GOOS` 交叉编译和配置生成的单元测试，没有真机验收。

## 目录

```text
internal/svcmgr/
├── config.go   服务配置生成、-conf 校验
├── env.go      当前用户/可执行文件/sudo 发起者，Linux 自动 sudo
├── daemon.go   -d / -stop 流程
├── program.go  Windows 服务接口（service.Interface）
└── config_test.go
```
