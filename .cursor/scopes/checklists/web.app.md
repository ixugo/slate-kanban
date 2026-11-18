# 网页应用骨架需求质量清单

用途：`web.app.md` 的自检与评审  
所有者：AI 生成，用户终审

## 完整性

- [ ] CHK001 - 路由表是否与 `AppRoutes.tsx` 一致（无 `/login`）？
- [ ] CHK002 - 实时流事件类型（ready / board / reset / error）与 `useBoardLive.ts` 处理是否一一对应？
- [ ] CHK003 - 五种语言、错误 `reason` 翻译、日期时间格式化是否都在 scope 或 locales 测试里覆盖？
- [ ] CHK004 - 开发代理路径是否覆盖 `vite.config.ts` 中的全部 `proxy` 项？

## 清晰度

- [ ] CHK005 - 合并刷新 150 ms、重连 5 s 等常量是否与源码常量同名同值？
- [ ] CHK006 - 乐观更新（看板页拖卡片）失败回滚范围是否在 `web.board.md` 写清？

## 一致性

- [ ] CHK007 - `lib/api/types.ts` 字段名与 Go JSON 是否仍由契约测试或 review 保证？
- [ ] CHK008 - 语言代码与 `normalizeLanguage` / 后端 `normalizeLocale` 是否一致？
- [ ] CHK009 - 文档是否仍声明「权限在服务端」，前端不靠隐藏按钮当安全？

## 免登录与实时

- [ ] CHK010 - 文档与 `client.ts` 是否一致：**无任何**令牌、登录页、`Authorization`？
- [ ] CHK011 - 是否说明 SSE 用 **EventSource** + **Last-Event-ID**，而非 fetch 手动解析？

## 轻量实现

- [ ] CHK012 - 是否标明 i18n / toast / Markdown 为自研，且已不依赖 i18next、sonner、react-markdown？
- [ ] CHK013 - 顶栏语言/看板为 `<select>`、主题为循环按钮，`TooltipProvider` 仅 `BoardPage`？
- [ ] CHK014 - 是否说明 `cn` 包**不**合并冲突 Tailwind 类？

## 体积与构建

- [ ] CHK015 - `check-size.mjs` 统计规则（script + modulepreload 的 js）是否与脚本实现一致？
- [ ] CHK016 - 预算数字是否与最近一次 `web/build` 输出一致（或注明测量日期）？

## 边界

- [ ] CHK017 - Markdown / 评论 XSS 策略是否与 `markdown.tsx` 测试一致？
- [ ] CHK018 - 看板已删（404）、`localStorage` 不可用、窄屏顶栏 select 是否在用例或 UI 上可接受？
- [ ] CHK019 - jsdom 下 `EventSource` 是否靠 `fakeEventSource` 与全局安装，避免进看板测试崩溃？