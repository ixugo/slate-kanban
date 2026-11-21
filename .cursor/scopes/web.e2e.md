# 浏览器端到端验收（e2e-strict）

## 需求目标

拖动、实时同步、布局、光标位置这些要真浏览器才能验证的东西，jsdom 里的单元测试看不见。
项目只维护一套浏览器端到端测试：`web/.e2e-strict/`（用 `e2e-strict` skill 生成，保存成固定的 Playwright 脚本，回放时不用模型、不自动修复）。

原来还有一套 `web/e2e`（旧的 Playwright 冒烟用例），已经按要求删除，只保留这一套。

## 运行

```bash
make -C web/.e2e-strict run      # 在仓库根目录；先构建前端，再编译带前端的程序，用临时目录和全新数据库启动，跑完停掉
make -C web/.e2e-strict headed   # 显示浏览器
make -C web/.e2e-strict report   # 打开最近一次报告
```

- 用本机 Google Chrome，浏览器语言固定为 `en-US`，所以用例里的按钮文字是英文。
- 每次运行都用独立的临时配置目录（`-conf`），不碰 `~/.slate-kanban`，也不占 7070 端口。
- 每个用例自己通过 REST 准备数据，用例之间不共享数据；接口请求出现非预期的失败会让用例失败。
- 证据（trace、截图、HTML）放在 `web/.e2e-strict/runs/`，只在本地，未脱敏，不要上传。
- `web/.e2e-strict/` 不提交到 git（已加入 `.git/info/exclude`），换机器时需要重新生成。

## 当前覆盖

见 `web/.e2e-strict/coverage.md`：标签的新建、改名、删除、重名；磁贴上没有封面图、列里很多任务时磁贴不被压扁；描述自动保存；评论工具栏；添加任务聚焦；属性按钮保存；手机宽度无横向滚动；任务编号显示；评论框光标行高。

## 旧套件删除后没有迁移的覆盖点

这些在旧套件里有真浏览器用例，现在只剩单元测试或没有自动化，以后出问题时需要优先补进 `.e2e-strict`：

| 内容 | 现在的保障 |
|------|-----------|
| 跨列拖动、同列换序、拖进收起的「完成」窄边 | `collision.test.ts`、`ColumnView.test.tsx` 单元测试，没有真实鼠标用例 |
| 搜索框语法、筛选弹层 | 单元和集成测试 |
| 规则触发（拖进列自动加标签、系统留言） | 服务端测试 |
| 两个窗口实时同步 | 服务端 SSE 测试和前端假 `EventSource` 测试 |
| 五种语言切换 | `locales.test.ts` 检查文案齐全，没有切换后 `<html lang>` 的真实检查 |
| 新建/删除项目（输入名称才能删） | 组件测试 |
| 导出下载 | 接口测试和组件测试 |
| 编辑器输入法（CDP `Input.imeSetComposition`）、列表 Tab/退格、撤销重做、任务框点击、粘贴图片上传 | `LiveEditor.test.tsx`、`render.test.ts` 等单元测试，没有真浏览器用例 |

## 写用例时要注意的地方

- 准备数据用 REST（`helpers.mjs`），界面上要验证的那一步才走界面。
- 断言服务端的结果用 `expect.poll`，因为界面上的乐观更新比请求先完成。
- dnd-kit 在松手后约 50 毫秒里会吞掉点击；拖完以后先刷新页面或等界面状态变化，再点。
- 同列换序、窄边落点这类问题的原因和修复记录：列的落点编号用 `column-<id>` 字符串（`dnd.ts` 的 `columnDropId`），收起的窄边用 `collision.ts` 的 `boardCollision` 直接选中。
