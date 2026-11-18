# 界面设计系统（苹果风格）

## 需求目标

用户要求按苹果设计原则（`apple-design` skill）美化整个网页：深浅两套主题都要好看，动效有手感，同时不能让首屏变大。
本文件只讲**视觉与动效的统一规则**；各页面的结构和行为见 `web.app.md`、`web.board.md`、`web.settings.md`、`web.agents.md`。

做法的核心是把"好看"落成**令牌和少量工具类**，页面里不再随手写颜色、圆角和曲线：令牌都在 `web/src/index.css`，基础组件在 `web/src/components/ui/`。

## 问题场景

| 编号 | 问题场景 | 不解决的后果 | 解决方案 | 状态 |
|------|----------|-------------|---------|------|
| 1 | 原来是直角加细线框，各处各写各的 | 页面之间不像一个产品；换主题要逐处改 | 颜色全部用 oklch 令牌（`--background`、`--card`、`--primary`、`--column`、`--field` 等），浅色是浅灰底配纯白卡片，深色是近黑底配提亮的卡片，不用纯黑纯白；系统蓝是唯一强调色，橙色（`--signal`）只表示"需要注意" | 实施 |
| 2 | 按钮、标签、页签的形状不统一 | 同一层级的控件长得不一样 | 大控件用胶囊（`rounded-full`）：按钮、徽标、页签、开关；输入框和下拉选择框 `rounded-lg`；气泡、下拉菜单、提示 `rounded-2xl`；对话框 `rounded-3xl`；字号只用四档令牌 `text-ui` 13px、`text-body` 15px、`text-title` 20px、`text-display` 28px，字号越大字距和行距越紧 | 实施 |
| 3 | 圆角嵌套时看起来不顺 | 内外圆心对不上，角落显得"鼓" | 同心圆角：外圆角 = 内圆角 + 内边距。例如卡片 14px，列内边距 6px，列就是 20px（`rounded-[1.25rem]`） | 实施 |
| 4 | 顶栏、弹窗和下拉要有层次 | 层叠后分不清谁在上面 | 两个工具类：`material-bar`（顶栏：背景 72% 透明度，模糊 24px、饱和度 180%）、`material-panel`（弹窗、下拉、气泡：背景 80%，模糊 40px，加 `--elev-float` 阴影和顶部 0.5px 的内高光）；不支持 `backdrop-filter` 时回退成不透明背景 | 实施 |
| 5 | 动效要有手感，还能被打断 | 点一下要等动画走完；回弹到处都是显得浮夸 | 全部用 CSS `transition`（天然可被打断）；曲线用 `linear()` 近似弹簧：`ease-spring`（临界阻尼，没有回弹，默认）、`ease-spring-bounce`（只给"被甩出去落下"这类有惯性的场合）、`ease-sheet`（弹层和列宽）；按下缩到 0.97；拖动中的卡片放大 1.03 并倾斜 1.2° | 实施 |
| 6 | 系统里开了无障碍选项 | 动效晕、透明度看不清、对比度不够 | `prefers-reduced-motion`：弹出层进入/退出的位移和缩放清零，所有过渡和动画缩成 120ms 的淡入淡出，呼吸圆点停掉；`prefers-reduced-transparency`：材质去掉模糊、换成不透明背景；`prefers-contrast: more`：材质不透明并加 1px 前景色描边 | 实施 |
| 7 | 开关要像 iOS，又要同时适配深浅 | 深色下关闭状态看不出来 | `Switch` 是 44×26px 的胶囊轨道，开启为绿色（`--success`），关闭是半透明前景色（深色下是白色 22%），滑块白色带阴影，按住时变长、松开用 `ease-spring-bounce` 弹回；新建项目对话框的模板开关、卡片里的"仅人工"、规则列表的启用开关都用它 | 实施 |
| 8 | 不能为了好看拉大首屏 | 违背"前端尽量小" | 不下载网页字体（系统字体，苹果设备上就是 SF，按语言挑汉字/假名/谚文字体）；不引动效库，全部 CSS；首屏 JS gzip 118.8 kB（预算 130），整个 `dist` 250.0 kB（预算 300） | 实施 |

## 约束：`cn` 不会去重冲突的类名

`cn` 来自 npm 包 `cn`，**不像 `tailwind-merge` 那样去掉同属性的冲突类**（见 `web.app.md` 第 10 条）。所以：

- 想改基础组件的外观，直接改 `components/ui/*.tsx` 里的基础类串，不要在调用处再叠一个同属性的类；
- 需要和基础样式完全不同的控件时，调用处用原生元素（例如看板搜索框和卡片标题输入框）；
- 同一属性要覆盖时，用带断点前缀的类并保证前缀相同。

## 关键令牌

| 类别 | 名称 | 说明 |
|------|------|------|
| 材质 | `material-bar`、`material-panel` | 工具类，定义在 `index.css`；高光是内阴影 `--glass-highlight` |
| 曲线 | `--ease-spring`、`--ease-spring-bounce`、`--ease-sheet` | `@theme` 里的变量，类名 `ease-spring` 等 |
| 阴影 | `shadow-card`、`shadow-card-hover`、`shadow-float` | 对应 `--elev-card`、`--elev-card-hover`、`--elev-float` |
| 圆角 | `--radius` 派生的 `rounded-sm … 4xl` | 同心圆角按"外 = 内 + 内边距"手算 |
| 语义色 | `--agent`、`--human`、`--p-low/mid/high`、`--success`、`--signal` | 卡片上的领取者、优先级圆点、连接状态 |

## 文件

| 路径 | 内容 |
|------|------|
| `web/src/index.css` | 令牌、材质工具类、曲线、无障碍媒体查询、滚动条 |
| `web/src/components/ui/*.tsx` | 按钮、输入、开关、页签、弹窗、下拉、气泡等基础组件（vendored shadcn，已改成新风格；不要求逐函数注释） |
| `web/src/app/*.tsx` | 顶栏、标志、404、加载条 |
| `web/src/features/**` | 各页面按上述规则使用令牌 |
| `web/public/favicon.svg` | 图标，和顶栏标志一致 |

## 验证计划

| 项 | 做法 | 结果 |
|----|------|------|
| 类型、静态检查、单元测试 | `npx tsc -b`、`npx oxlint`、`npx vitest run` | 通过（354 个用例） |
| 构建与体积 | `npm run build`（含 `scripts/check-size.mjs`） | 首屏 118.8 / 130 kB，`dist` 250.0 / 300 kB |
| 真实渲染（桌面 1280×800） | 看板页、卡片弹窗（浅色、深色都看过）；项目列表、新建项目对话框、设置（常规、自动化、规则编辑器）、Agent 页（深色）；设置的「列」页（浅色） | 已看过截图；每页至少看过一个主题 |
| 生产构建 | 用 `make build` 得到的单文件程序，在浏览器里新建项目、跳转到看板 | 研发流程模板得到 5 列，「完成」列是 48px 窄边；首页响应带 `Content-Encoding: gzip` |
| 真实渲染（手机 390×844，浅色） | 看板页（列按整屏吸附）、卡片弹窗（单栏） | 已看过截图 |
| 卡片标题自适应高度 | 在浏览器里量 `clientHeight` 与 `scrollHeight` | 两者相等，没有多出的滚动条 |

## 限制

- 没有在 Safari、Firefox 和真实触屏设备上逐页看过；`backdrop-filter` 不支持时的回退（不透明背景）只在代码里保证，没有实测。
- 没有对每一页逐一测试 `prefers-reduced-motion`、`prefers-reduced-transparency`、`prefers-contrast` 的效果；它们是 `index.css` 里的全局规则。
- 顶栏的毛玻璃在看板页不明显，因为看板页内容不在顶栏下面滚动；设置页、Agent 页等整页滚动的页面才能看出来。
