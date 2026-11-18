# Agent 接入需求质量清单
用途: agent.skill.go.md 的需求质量自检
所有者: AI 生成，用户终审

## 完整性
- [ ] CHK001 - 项目路径"有就不问、没有才问一次、写回看板"的全过程是否都有对应工具和规则？[Gap]
- [ ] CHK002 - 阻塞守候、host 工具超时很短、host 不能保持会话三种环境是否都给了做法？[Gap]
- [ ] CHK003 - 来活之后的子 agent、worktree、分支、续租、交接到审查、返工是否都写清？[Gap]
- [ ] CHK004 - 三种 host 的 MCP 安装和 skill 安装是否都给了可直接复制的步骤？[Gap]
- [ ] CHK005 - agent 没装 skill 就连上 MCP 时，有没有办法让它自己装？[Gap]

## 清晰度
- [ ] CHK006 - "agent 只处理哪些列""做完交给谁""哪些字段不能改"是否明确？[Ambiguity]
- [ ] CHK007 - 并发上限、续租间隔、worktree 和分支的命名是否量化？它们是建议还是强制是否说清？[Ambiguity]
- [ ] CHK008 - 每个服务端错误名对应的处理办法是否列清楚？[Ambiguity]

## 一致性
- [ ] CHK009 - skill 里的工具名、错误名、字段名与服务端实际一致吗，新增或改名时有没有测试提醒？[Conflict]
- [ ] CHK010 - 单张用 `next_card`、多张用 `list_cards` 加 `claim_card`，与 `next_card` "优先给已持有的卡片"的行为是否一致？[Conflict]
- [ ] CHK011 - skill 里"不要自己拖进完成列"与服务端的权限（agent 不能放进领取列、`finish_card` 去下一列）是否一致？[Conflict]

## 边界场景
- [ ] CHK012 - 免认证的两个地址会不会泄露令牌或其他敏感信息？[Edge Case]
- [ ] CHK013 - Host 或转发头里带恶意内容时会不会被写进说明页？[Edge Case]
- [ ] CHK014 - 令牌明文会不会被写进会提交的文件或日志？[Edge Case]
- [ ] CHK015 - 看板的项目路径已有值但和当前目录不一致时，agent 会不会擅自改？[Edge Case]
- [ ] CHK016 - skill 被编译进二进制后，仓库里的文件、HTTP 下载、MCP 资源三者是否一致？[Edge Case]
- [ ] CHK017 - 服务放在反向代理后面（Host 是域名、协议是 https）时，说明里的地址对不对？[Edge Case]
