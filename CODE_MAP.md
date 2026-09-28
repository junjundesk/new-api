# 代码地图 CODE_MAP.md — new-api

> 由 codegraph 于 2026-09-04 生成。指标基于静态代码图谱：
> 1865 文件、10265 函数、1455 类、363k 行（Go 后端 796 + TypeScript 前端 1057，其余为配置/脚本）。
> 源码树内未维护独立的架构文档；本文是索引而非权威定义，改代码时应同步更新本文。

---

## 1. 总览

AI API 网关（Go + React），把 40+ 上游 AI 提供方统一成一个类 OpenAI 接口，附带用户管理、计费、限流、管理面板。依赖：Gin、GORM v2、Redis、SQLite/MySQL/PostgreSQL、JWT、WebAuthn、OAuth、Bun + React 19 + Base UI + Tailwind。

```
┌────────────┐    ┌──────────────────────────────────────────────┐
│   Client   │───▶│  router/ (Gin 路由托管)                        │
└────────────┘    │  ├── web-router   /api/status, 控制台页面        │
                  │  ├── relay-router  /v1/*  → 中转集群            │
                  │  ├── channel-router /api/channel/* 管理          │
                  │  └── api-router    账单/令牌/密钥/系统设置        │
                  └──────────┬───────────────────────────────────┘
                             ▼
                  middleware/ (认证·限流·审计·分发)
                  ├── TokenAuth/Distribute ──▶ relay/ 中转链
                  │                              ├── common_handler（预扣费/币值换算/用量统计）
                  │                              ├── helper / constant / common
                  │                              └── channel/*（40+ 提供方适配器）──▶ 上游
                  ├── model/ Service 层逻辑（预扣费、token 换算、任务轮询）
                  ├── model/ GORM 数据层（用户/令牌/日志/渠道/计费）
                  └── pkg/  cachex · ionet · perf_metrics · billingexpr
```

分层惯例：`router → middleware → controller/service → model`；中转流量在 `middleware.Distribute` 之后直接进入 `relay/`。

---

## 2. 后端模块（Go）

| 模块 | 文件 | 函数 | 职责 |
|---|---|---|---|
| `router/` | 10 | 14 | Gin 路由挂载（main.go、relay-router.go、web-router.go、channel-router.go、api-router.go） |
| `middleware/` | 33 | 148 | TokenAuth、Distribute（渠道分发+预扣费）、限流、CORS、审计；热点 `getModelRequest`(65) |
| `controller/` | 89 | 736 | HTTP 处理器，业务公式解耦在 service；复杂度热点 `ManageMultiKeys`(87)、`FetchUpstreamRatios`(85)、`testChannel`(84) |
| `service/` | 90 | 788 | 业务逻辑：token 计数、文本/图像/视频计费、任务轮询（task_polling.go）、日志生成；热点 `EstimateRequestToken`(35)、`getImageToken`(44) |
| `model/` | 77 | 894 | GORM 模型 + 数据访问（用户/令牌/日志/渠道/任务/订阅/选项）；热点 `updateOptionMap`(154)、`updatePricing`(63) |
| `relay/` | 242 | 1607 | 中转过户链路：请求转换、流式转发、任务平台（Midjourney 等）；热点 `RelayMidjourneySubmit`(74)、`applyOperations`(84) |
| 　└ `relay/channel/` | 192 | 1187 | 提供方适配器（见 §4） |
| 　└ `relay/channel/task/` | — | — | 异步任务平台：ali/doubao/gemini/hailuo/jimeng/kling/sora/suno/vertex/vidu |
| `relay/helper` `common` `constant` | — | — | 中转工具：请求校验、token 换算、格式类型常量 |
| `relaykit/` | — | — | **独立 Go module**，relayconvert（OpenAI/Claude/Gemini/Responses 互转）、types、dto、reasonmap、reasoning；不得引入根 module（见 AGENTS.md 强制校验：`cd relaykit && GOWORK=off go build ./...`） |
| `setting/` | 52 | 248 | 运行期配置：model/ratio/billing/system/operation/console/perf；热点 `getHardcodedCompletionModelRatio`(60) |
| `common/` | 58 | 343 | 共享工具：JSON 包装（Marshal/Unmarshal 统一走 `common.*`）、quota_math（配额换算防溢出）、限流 Lua、sys_log |
| `constant/` | 13 | 1 | 渠道类型/API 类型/上下文键常量 |
| `dto/` | 4 | 31(类) | 请求/响应 DTO |
| `types/` | 3 | 28 | relay 格式类型、PriceData（计费倍率，AddOtherRatio 校验）、set.go |
| `oauth/` | 9 | 84 | GitHub/Discord/OIDC 等 OAuth 提供方、访问策略求值 |
| `pkg/billingexpr/` | 7 | 80 | 表达式计费引擎（见 `pkg/billingexpr/expr.md`） |
| `pkg/cachex/` | 3 | 22 | 缓存封装（Get/DeleteByPrefix/DeleteMany） |
| `pkg/ionet/` | 6 | 39 | 容器/极网络工具（io.net 客户端） |
| `pkg/perf_metrics/` | 4 | 40 | 性能指标聚合（Redis + 查询） |
| `i18n/` | 5 | 715 | go-i18n，en/zh 双语 |
| `logger/` | 1 | 10 | 日志/配额日志（SetupLogger、LogQuota） |
| `electron/` | 4(TS) | 13 | 桌面外壳（更新、错误分析） |

---

## 3. 请求生命周期（中转主链路）

1. **路由**：`/v1/*` → `relay-router.go` 的 `SetRelayRouter`；`/api/*` 管理与控制台走 web/api 路由。
2. **鉴权**：`middleware.TokenAuth` 解析 JWT / API Key，写入用户上下文。
3. **分发**：`middleware.Distribute` 按模型+分组选渠道，发起**预扣费**，把请求交给 `relay/common_handler`。
4. **转换**：`relay/helper` 校验请求（`max_tokens` 等 `maxTokensLimit` 上限）→ channel 适配器把 OpenAI 格式转成上游格式（relaykit/relayconvert 负责标准格式互转）。
5. **转发**：适配器请求上游（HTTP/流式 SSE/WebSocket/任务提交），流式场景实时回读。
6. **结算**：`relaykit`/`service` 做 token 换算与配额计算（`common.QuotaFromFloat/Round` 等防溢出助手）→ 写 consume/log 日志 → 预扣费与实扣差额退补（settle）。
7. **落库**：用量日志、任务状态、渠道健康度由 `model` + 后台协程持久化。

异步任务类（图/视频/音乐）走 `relay/channel/task/*`：提交 → 轮询（`service/task_polling.go`）→ 结算。

---

## 4. 渠道适配器（relay/channel/）

按渠道类型分组，均在 `channel.go` 注册 `GetAdaptor`：

| 组 | 适配器 |
|---|---|
| 标准 OpenAI | openai, openrouter, perplexity, moonshot, deepseek, lingyiwanwu, minimax, mistral, mokaai, siliconflow, zhipu, zhipu_4v, cohere, ai360, xai, codex, newapi, sub2api, submodel, jina, jimeng, xinference, xunfei |
| Anthropic | claude |
| Google | gemini, palm, vertex |
| 云/平台 | aws (Bedrock), azure(Azure 库内), ali, baidu, baidu_v2, tencent, volcengine, cloudflare, ollama（本地）, dify, coze, replicate |
| 任务类 | task/ali, task/doubao, task/gemini, task/hailuo, task/jimeng, task/kling, task/sora, task/suno, task/vertex, task/vidu |

---

## 5. 前端（web/src，React 19 + TS）

`web/AGENTS.md` 为前端权威规范；包管理用 bun。结构：

```
web/src/
├── features/      功能域（每个域内 self：api/、components/、hooks/、lib/、types/）
│   ├── auth       登录/注册/OAuth/Passkey/OTP/找回密码
│   ├── channels   渠道管理（列表/新建抽屉/测试）
│   ├── keys / users / wallet / usage-logs / redemption-codes
│   ├── models     模型定价（动态计费表达式预览）
│   ├── dashboard  首页看板（图表）
│   ├── playground / chat  在线体验聊天
│   ├── system-settings  系统设置（分域注册 section-registry，含 auth/billing/content/models/operations/security/site）
│   ├── subscriptions / pricing  订阅与定价页
│   └── home / about / errors / legal / rankings / setup / performance / system-info / profile
├── components/   共享组件（ui/* 为 Base UI 封装，data-table/*、layout/*、ai-elements/*）
├── lib/          API 客户端、格式化、权限、路由工具
├── hooks/ stores/ contexts/  顶层状态
└── i18n/         locales/{en,zh,zh-TW,fr,ru,ja,vi}.json（平铺 JSON，英文原文为 key）
```

复杂度热点：`DetailsDialog`（usage-logs, 144）、`ChannelMutateDrawer`（channels, 143）。

---

## 6. 热路径（被调用最频繁的函数）

| 函数 | 文件 | 直接调用 | 传递调用 |
|---|---|---|---|
| `SysLog` | common/sys_log.go | 172 | 1364 |
| `Equal` | model/task.go | 654 | 47 |
| `Unmarshal` | relay/channel/volcengine/protocols.go | 317 | 853 |
| `Unlock` | controller/user.go | 108 | 1347 |
| `Close` | relaykit/types/file_source.go | 170 | 1149 |
| `Marshal` | relay/channel/volcengine/protocols.go | 272 | 641 |
| `Bytes` / `NewReader` / `Read` | common/body_storage.go | 40 / 120 / 13 | 1275 / 780 / 822 |
| `SysError` | common/sys_log.go | 69 | 372 |

---

## 7. 已知架构关注点

- **循环依赖**：codegraph 检出 **39 处** circular dependency。改路由/模型/服务时优先保持依赖单向。
- **计费安全**：配额计算必须走 `common/quota_math.go` 的 `QuotaFromFloat/Round/FromDecimal`（及 `*Checked`），禁止裸 `int(...)` 转换；乘法器必须 `AddOtherRatio` 校验；预扣费失败不得静默回绕（详见 AGENTS.md「Billing safety invariants」）。
- **json 唯一入口**：业务代码禁用 `encoding/json` 的 Marshal/Unmarshal 调用，统一 `common/json.go`。
- **DB 三端兼容**：SQLite / MySQL / PostgreSQL 同跑；行锁用 `model.lockForUpdate(tx)`；原始 SQL 按方言分支。
- **relaykit 独立构建**：改 relaykit 影响面时 `cd relaykit && GOWORK=off go build ./...` 必须通过。

---

*Generate 自 `codegraph_generate_architecture_doc`（topN=15）。*