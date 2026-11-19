# REST 接口与实时流需求质量清单
用途: api.rest.go.md 的需求质量自检
所有者: AI 生成，用户终审

## 完整性
- [x] CHK001 - 每个 Core 对外方法是否都有对应的路由（含 ExportBoard）？[Gap]
- [x] CHK002 - 错误响应的格式和状态码来源是否定义？[Gap]
- [x] CHK003 - 流在看板不存在、看板中途被删、起点过旧三种情况下的行为是否定义？[Gap]
- [x] CHK004 - 流在单次最长连接到期后如何续连是否定义？[Gap]
- [x] CHK005 - 请求体过大、类型不对的返回是否定义？[Gap]

## 清晰度
- [x] CHK006 - 心跳间隔、单次连接最长时间、请求体上限是否量化？[Ambiguity]
- [x] CHK007 - 续传位置同时给了 after 和 Last-Event-ID 时以谁为准是否明确？[Ambiguity]
- [x] CHK008 - 哪些接口只有人能调用、哪些 agent 也能调用是否列清楚？[Ambiguity]

## 一致性
- [x] CHK009 - 路由里的"谁能调用"与 Core 里的权限判断是否一致？[Conflict]
- [x] CHK010 - 阻塞等待的上限与常见反向代理超时是否匹配？[Conflict]
- [x] CHK011 - 流里的事件 JSON 与 REST 事件列表、agent 拉取结果是否是同一种格式？[Conflict]

## 边界场景
- [x] CHK012 - 请求体或查询参数里带同名字段能否覆盖路径里的编号？[Edge Case]
- [x] CHK013 - 导出附件文件名是否防止响应头注入？[Edge Case]
- [x] CHK014 - 非法 export format 是否返回 400？[Edge Case]
- [x] CHK015 - 没有请求体的 POST（领取、释放）能否正常工作？[Edge Case]
- [x] CHK016 - 路径里的编号不是数字、是 0 或负数时的返回是否定义？[Edge Case]
- [x] CHK017 - 流经过框架的日志包装后还能不能即时送达？[Edge Case]
