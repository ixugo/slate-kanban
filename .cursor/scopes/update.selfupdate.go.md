# 自动更新（internal/web/api/updateapi、web/src/features/update）

## 需求目标

用户打开网页时，服务端去 GitHub release 比较版本，有新版本就弹窗提示；用户可以忽略这个版本，或者立即更新：服务端下载对应平台的包，校验后替换本地程序。

## 决策

| 项 | 决定 |
|---|---|
| 谁会被提示 | 只有官方 release 构建（发布工作流把 `gitBranch` 写成 `release`，`main.go` 据此设置 `Runtime.Official`）；本机编译的版本、容器里的程序不检查或只提示不更新 |
| 查询 | 网页每次打开请求 `GET /api/v1/update`；服务端请求 `releases/latest`，成功缓存 1 小时，失败 5 分钟内不重试，失败只记日志、按没有更新返回 |
| 忽略 | 网页把被忽略的版本号写进 localStorage `slate.update.ignored`，小于等于它的版本不再弹窗；不同的电脑仍会提醒 |
| 包的选择 | 按 `GOOS/GOARCH` 找 `slate_<tag>_<系统>_<架构>.tar.gz`（windows 为 `.zip`）和 `checksums.txt`，规则和 `.github/workflows/release.yml` 一致；包地址必须在 `https://github.com/ixugo/slate-kanban/releases/download/` 下 |
| 校验 | 下载的包必须和 `checksums.txt` 里的 sha256 一致，否则放弃，不改动现有程序 |
| 替换 | 临时文件写在程序所在目录；类 Unix 直接改名覆盖，windows 先把旧程序改名成 `.old` 再放新的，放不上去就改回来；不做备份 |
| 重启 | 不自动重启：替换成功后网页提示用户重启 Slate 并刷新页面，因为不知道程序由谁守护 |
| 认证 | 接口在 `/api/v1` 下，启用口令时和其他接口一样需要认证 |
| 不能更新时 | 容器里、release 里缺当前平台的包时，`can_apply` 为假，弹窗只给「去下载」链接 |

## 风险与已知缺口

| # | 风险 | 处理 |
|---|---|---|
| 1 | 程序所在目录没有写权限 | 下载前就失败，提示没有权限写入，不改动现有程序 |
| 2 | 同时点两次更新 | 同一时刻只允许一次，另一次返回冲突 |
| 3 | 包被篡改或下载不完整 | sha256 校验不过就放弃；包和程序都有大小上限 |
| 4 | GitHub 不通或被限流 | 按没有更新处理，网页不报错 |
| 5 | 工作流和自动更新还没有在真实的 GitHub release 上联调过 | 单元测试用假的 GitHub 覆盖了下载、校验、替换，第一次真实发布后需要实测一次 |

## 验收清单

- [x] 版本比较、缓存、失败不报错、非官方不查询、容器只提示
- [x] 类 Unix 和 windows 的替换，校验不过不改程序且不留临时文件
- [x] 网页：弹窗、忽略、立即更新、失败重试、去下载
- [ ] 真实 release 上联调
