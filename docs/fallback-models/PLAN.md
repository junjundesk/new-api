# 密钥级兜底模型（Model Fallback）实施方案

> 状态：P0 已实现（密钥配置、relay 兜底链路、按实际模型计费、日志「兜」徽标与管理员尝试明细）  
> 范围：`/keys` 密钥编辑抽屉、`/usage-logs/common` 使用日志、relay 主链路与计费  
> 预览：`docs/fallback-models/preview.html`（`?view=keys` / `?view=logs`）  
> 图片（含标注）：`docs/fallback-models/preview-keys.png`、`docs/fallback-models/preview-logs.png`  
> 图片（纯净版）：`docs/fallback-models/preview-keys-clean.png`、`docs/fallback-models/preview-logs-clean.png`

---

## 1. 需求结论（先对齐语义）

| 项目 | 结论 |
| --- | --- |
| 配置入口 | 密钥编辑抽屉新增独立分区「兜底模型」，位于「模型映射」之后、「额度设置」之前 |
| 配置粒度 | 每个 API Key 一份有序兜底列表；列表项 = `{ source_model?, group, model }`，`source_model` 为空表示对所有原模型生效 |
| 多兜底 | 同一个原模型可配置多条兜底（按列表顺序依次尝试）；也可配置 `source_model` 为空的通用兜底（最多 5 条） |
| 分组范围 | 分组下拉只列「当前用户可访问」的具体分组（排除 `auto` 与 `chain:*`），模型下拉按所选分组实时拉取 |
| 触发条件 | 原模型请求失败（非 2xx、无可用渠道、上游超时/空响应），且本次请求**没有任何输出**（未写出响应体、流式未开始输出、completion tokens = 0） |
| 不触发 | 客户端错误（400/401/403 鉴权/413/422）、敏感词、余额不足、内容审核、用户主动取消、已经开始的流式响应、渠道测试 |
| 计费 | 按**实际成功请求的模型**计费；每次兜底尝试用目标模型的模型倍率 + 目标分组倍率重新定价，结算按最终模型的真实用量 |
| 日志 | 消费日志的 `model_name` 记录实际模型；模型列显示「兜」徽标，悬浮/详情展示「原模型 → 兜底模型」链路 |
| 与模型映射的关系 | 先执行令牌模型映射（客户端模型名 → 目标模型），渠道映射仍优先；密钥兜底发生在所有常规渠道重试与渠道映射候选都失败之后 |
| 本期不支持 | 图片/音频/视频/任务类、OpenAI Realtime（WebSocket）、已开始的流式请求（P1 再评估） |

---

## 2. 现状盘点与可复用能力

| 现有能力 | 位置 | 与本需求的关系 |
| --- | --- | --- |
| 令牌模型映射（单跳） | `middleware/token_model_mapping.go`、`setting.TokenModelMapping` | 在鉴权后、选渠道前改写模型；计费天然按目标模型。兜底是「失败后」的第二次改写，可复用同一套表单/校验风格 |
| 渠道模型映射多候选 | `relay/helper/model_mapped.go`、`model_failover.go`、`RelayInfo.ModelCandidate` | 已有「同一渠道内换候选模型」的状态机，可作为兜底状态机的参考；但候选只在同一渠道内，不能跨分组、也不能从密钥页配置 |
| auto 分组 / 跨分组重试 | `service/channel_select.go`、`RetryParam` | 可复用渠道选择与分组切换逻辑；但跨分组重试不会换模型，且只在 `auto` 分组生效 |
| 分组链 | `Token.Group = chain:*` | 与兜底互不冲突；兜底目标仍按单分组选择 |
| 计费会话 | `service.PreConsumeBilling` / `BillingSession` / `SettleBilling` | 已支持「预扣 → 补扣/退款」，兜底换模型只需重新定价并 Reserve 差额 |
| 中间失败日志隐藏 | `processChannelError(..., adminOnly)` | 兜底尝试的中间失败同样只进管理员日志，用户只看到最终结果 |
| 模型价格/倍率解析 | `relay/helper/price.go: ModelPriceHelper` | 重新定价直接复用；tiered_expr 会重建 `TieredBillingSnapshot` |
| 日志 other 扩展 | `service/log_info_generate.go` | 新增 `other.fallback`（用户可见）与 `admin_info.fallback_attempts`（管理员） |

**结论：不新建独立网关链路**，在 `controller/relay.go` 的现有重试循环之后追加「兜底阶段」，复用渠道选择、定价、计费会话与日志设施。

---

## 3. 数据模型

### 3.1 新增字段（`model/token.go`）

```go
type Token struct {
    // ... existing fields
    FallbackEnabled bool   `json:"fallback_enabled"`
    FallbackModels  string `json:"-" gorm:"type:text"`
}
```

存储格式（数组有序，索引即优先级）：

```json
[
  { "source_model": "gpt-5", "group": "vip", "model": "claude-sonnet-4-5" },
  { "source_model": "gpt-5", "group": "azure", "model": "gpt-4.1-mini" },
  { "source_model": "", "group": "gemini", "model": "gemini-2.5-flash" }
]
```

配套方法（与 `GetModelMappings/SetModelMappings` 同风格）：

- `func (t *Token) GetFallbackModels() ([]setting.TokenFallbackModel, error)`
- `func (t *Token) SetFallbackModels(items []setting.TokenFallbackModel) error`（空列表写 `""`）
- `model.Token.Update()` 的 `Select(...)` 增加 `fallback_enabled`, `fallback_models`（当前是白名单更新，漏掉会静默不保存）
- 令牌缓存（Redis/GORM 缓存）会自动带上新字段；更新后沿用现有 `cacheSetToken` 失效逻辑

### 3.2 类型与校验（新增 `setting/token_fallback.go`）

```go
type TokenFallbackModel struct {
    SourceModel string `json:"source_model,omitempty"` // 空 = 对所有原模型生效
    Group       string `json:"group"`
    Model       string `json:"model"`
}

const MaxTokenFallbackModels = 5

func NormalizeTokenFallbackModels(items []TokenFallbackModel) ([]TokenFallbackModel, error)
```

校验分两层，避免 `setting` 包反向依赖 `service` 造成 import cycle：

**结构校验：`setting.NormalizeTokenFallbackModels`（只依赖 common）**

1. 最多 `MaxTokenFallbackModels` 条；允许 0 条（等于关闭）。
2. `source_model`、`group`、`model` 去首尾空格；`group`、`model` 不能为空，`source_model` 可为空。
3. 同一 `source_model + group + model` 不允许重复；`source_model` 非空时长度/非法字符校验与 `NormalizeTokenModelMappings` 一致。

**业务校验：`controller.setTokenFallbackModels`（保存时）**

4. `source_model` 非空时必须在该 Key 可访问的模型列表内（与模型映射使用同一可用模型集合）；它指**请求实际使用的模型**（令牌模型映射后的目标模型），不是客户端原始模型名。
5. 分组必须是用户可用的具体分组：`service.GroupInUserUsableGroups(userGroup, group)`，且不是 `auto` / `chain:*`。
6. 模型必须在该分组的启用模型列表中：`service.GetGroupsEnabledModels([]string{group})`。
7. 若令牌开启了 `model_limits_enabled`，兜底模型也必须在允许列表内。
8. 模型需有可用计费配置（`helper.HasModelBillingConfig`，自用模式/用户豁免逻辑与 `ListModels` 保持一致）；也可用相同规则在 `setting` 内联判断，避免 controller 直接依赖 relay/helper。

### 3.3 数据库兼容

- 只新增列，GORM `AutoMigrate` 在 SQLite / MySQL 5.7+ / PostgreSQL 9.6+ 均可 `ADD COLUMN`。
- 不加 `default:true` 之类布尔默认标签（遵循项目约定），默认值由 Go 零值与请求归一化保证。
- 旧数据 `fallback_models = ''`、`fallback_enabled = false`，行为完全不变。
- 导出/导入、订阅绑定等复制 Token 字段的代码需同步补字段（仓库内检索 `ModelMappings` 的所有赋值点）。

---

## 4. 后端实现

### 4.1 API 与鉴权

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/token/fallback/options?group=vip` | 返回该分组可用模型 `{ models: string[] }`；校验用户分组权限；支持搜索由前端本地过滤 |
| POST/PUT | `/api/token/`、`/api/token/:id` | 请求体新增 `fallback_enabled`、`fallback_models`；服务端归一化 + 校验后落库 |
| GET | `/api/token/:id` | 响应新增 `fallback_enabled`、`fallback_models`（`tokenResponse` 显式字段，Token 本体保持 `json:"-"`） |

实现点：

- `controller/token.go`：`tokenRequest` 增加 `FallbackModels *[]setting.TokenFallbackModel`；`tokenResponse` 增加字段与解析；`AddToken`/`UpdateToken` 写入；`buildMaskedTokenResponseWithUsage` 解析。
- 新增 `setTokenFallbackModels(c, token, items)`：归一化 + 权限/模型校验，失败返回 400 与可读错误（i18n）。
- `GetTokenFallbackOptions`：复用 `getTokenGroupModels` 的思路，但只针对单个具体分组：

```go
func GetTokenFallbackOptions(c *gin.Context) {
    group := strings.TrimSpace(c.Query("group"))
    userGroup, err := getTokenRequestUserGroup(c)
    if err != nil || group == "" || group == "auto" || strings.HasPrefix(group, "chain:") || !service.GroupInUserUsableGroups(userGroup, group) {
        common.ApiError(c, fmt.Errorf("group is not available"))
        return
    }
    models := service.GetGroupsEnabledModels([]string{group}) // + 计费配置过滤 + 排序
    common.ApiSuccess(c, gin.H{"models": models})
}
```

- 注意：`GetToken` 返回的是缓存/数据库 Token，`FallbackModels` 是 `json:"-"`，必须走 `tokenResponse` 显式字段，避免前端拿不到。
- 审计：`recordUserSecurityAudit` 可增加兜底配置摘要（`count`），不记录完整模型列表以免噪音。

### 4.2 请求上下文

- `constant/context_key.go` 新增 `ContextKeyTokenFallbackModels`。
- `middleware/auth.go: SetupContextForToken` 解析 `token.GetFallbackModels()` 并写入上下文（与 `token_model_mappings` 并列）。
- `relay/common/relay_info.go` 新增：

```go
type FallbackAttempt struct {
    Group      string
    Model      string
    ChannelId  int
    StatusCode int
    Error      string
}

type FallbackState struct {
    Configured    []setting.TokenFallbackModel
    OriginalModel string
    OriginalGroup string
    Attempts      []FallbackAttempt
    Used          *FallbackAttempt
}
```

`RelayInfo` 增加 `Fallback *FallbackState`；`GenRelayInfo` 之后由 `Relay()` 初始化（不要放进 `ChannelMeta`，它每次尝试都会重建）。

### 4.3 relay 主流程改造（`controller/relay.go`）

**第一步：抽出单次尝试函数**（纯重构，行为不变）

```go
func runRelayAttempt(c *gin.Context, relayFormat types.RelayFormat, relayInfo *relaycommon.RelayInfo,
    retryParam *service.RetryParam, reprice bool, tokens int, meta *types.TokenCountMeta) (*model.Channel, *types.NewAPIError)
```

内容即当前循环体：`getChannel` → `addUsedChannel` → （可选重定价）`PrepareBillingForSelectedChannel` → `GetBodyStorage` → 各 `relay.*Helper`。返回本次实际使用的 channel 与错误，供兜底阶段记录 attempt（渠道 ID、状态码）。主循环传 `reprice=false`；兜底传 `reprice=true`，并且**必须先选渠道再定价**，因为分组倍率与渠道专属模型倍率都取自刚选中的渠道上下文。

**第二步：主循环结束后追加兜底阶段**

```go
for ; retryParam.GetRetry() <= relayRetryLimit(c); retryParam.IncreaseRetry() {
    _, newAPIError = runRelayAttempt(...)
    if newAPIError == nil { return }
    // ... 现有 failover / processChannelError 逻辑保持不变
}

if newAPIError != nil && helper.ShouldTryFallback(c, relayInfo, newAPIError) {
    if used := runFallbackAttempts(c, relayFormat, relayInfo, request, tokens, meta); used {
        // 关键：必须清空错误，否则外层 defer 会把成功的兜底请求当成失败并退款
        newAPIError = nil
        relayInfo.LastError = nil
        return
    }
}
```

**第三步：`runFallbackAttempts`（建议放新文件 `controller/relay_fallback.go`）**

对每个条目按顺序：

1. **筛选目标**：只遍历 `SourceModel` 为空或等于当前 `relayInfo.OriginModelName` 的条目（保持配置顺序）；再做运行时权限与模型限制复核，不通过则记录 attempt 并跳过。
2. **切换目标状态**（关键，顺序不能乱）：
   - `relayInfo.OriginModelName = entry.Model`
   - `relayInfo.UsingGroup = entry.Group`
   - `common.SetContextKey(c, constant.ContextKeyAutoGroup, entry.Group)`（否则 `HandleGroupRatio` 会把 `UsingGroup` 改回上一次 auto 分组）
   - 清理 `ContextKeyAutoGroupIndex` / `ContextKeyAutoGroupRetryIndex`
   - `relayInfo.ModelCandidate = nil`、`relayInfo.IsModelMapped = false`、`relayInfo.UpstreamModelName = entry.Model`
   - `request.SetModelName(entry.Model)`
   - **保证 `relayInfo.ChannelMeta != nil`**（`getChannel` 在 `ChannelMeta == nil` 时会直接复用上下文里的旧渠道，不会重新选渠道；若为 nil 则先置为 `&relaycommon.ChannelMeta{}`，这是最容易踩的坑）
3. **先选渠道，再重新定价**：新建 `RetryParam{TokenGroup: entry.Group, ModelName: entry.Model, Retry: 0}`，在 `runRelayAttempt` 内先执行 `getChannel`（渠道会写入该分组的 `ChannelSettings` 与分组上下文，渠道专属倍率依赖它），再清空 `PriceData` / `TieredBillingSnapshot` / `BillingRequestInput`，调用 `helper.ModelPriceHelper(c, relayInfo, tokens, meta)`。定价失败（该模型未配置价格、表达式错误）→ 记录 attempt，跳过该兜底。
4. **预扣/补预留**：`service.PrepareBillingForSelectedChannel(c, relayInfo)`（同一次 attempt 只调用一次；更贵补预留、更便宜结算退款）。余额不足 → 记录并停止（返回原错误，避免用户看到与兜底价不符的 402）。
5. **执行**：调用 `runRelayAttempt` 的剩余步骤（读 body → 各 `relay.*Helper`）。每次兜底目标只尝试一次，不做渠道级重试（防止 5×N 放大）。实现上给 `runRelayAttempt` 加 `reprice bool` 参数：主循环传 false（沿用入口定价），兜底传 true（在选渠道后重定价）。
6. **成功**：写入 `relayInfo.Fallback.Used`，返回成功；调用方必须把外层 `newAPIError` 置为 nil，并清空 `relayInfo.LastError`，否则外层 defer 会误退款。
7. **失败**：记录 `FallbackAttempt{Group, Model, ChannelId, StatusCode, Error}`；调用 `processChannelError(..., adminOnly=true)`；继续下一个。
8. **全部失败**：恢复 `relayInfo.OriginModelName` 与 `c.Set("original_model", ...)` 为客户端原模型（`SetupContextForSelectedChannel` 会写入 `original_model`，不恢复会让最终错误日志把兜底模型当成原模型），返回最后一个错误或原错误（建议返回最后一个兜底错误，并在 message 中标注「兜底模型均已失败」；错误日志带完整链路）。退款仍由现有 defer 统一处理。

**触发判定 `helper.ShouldTryFallback`**

必须同时满足：

- `relayInfo.Fallback != nil && len(Configured) > 0`
- 存在 `SourceModel == "" || SourceModel == relayInfo.OriginModelName` 的匹配条目（按配置顺序保留）
- `newAPIError != nil`
- `!c.Writer.Written()`（非流式响应体未写出；流式未开始）
- `relayInfo.SendResponseCount == 0`（没有向下游发出任何响应块）
- `!relayInfo.IsChannelTest`
- relay 格式在白名单：OpenAI Chat/Completions、Claude Messages、Gemini、Responses、Embeddings（P0）
- 错误类型可兜底：
  - `ErrorCodeGetChannelFailed`（该分组无可用渠道）
  - `ErrorCodeBadResponseStatusCode` / 上游 5xx / 429 / 404 / 408 / 空响应
  - 上游连接失败、超时
- 明确排除：`ErrorCodeInvalidRequest`、`ErrorCodeAccessDenied`、`ErrorCodeSensitiveWordsDetected`、`ErrorCodeInsufficientQuota`、`ErrorCodeContentPolicy`、所有 `SkipRetry` 的客户端错误、4xx 中 400/401/403/413/422。

> 说明：`GetChannelFailed` 虽是 `SkipRetry`，但它正是「原模型不可用」的典型场景（分组下没有该模型渠道），因此兜底判定不直接复用 `shouldRetry`，使用独立白名单。

**流式关键点**：上游在写 SSE 前返回非 200 时 gin 尚未 `Written()`，可以兜底；一旦开始写 SSE 或已经下发任何字节，禁止兜底，避免响应拼接污染。

### 4.4 计费正确性

- `ModelPriceHelper` 使用 `info.OriginModelName` 取模型价格，所以必须先切模型再定价。
- `PrepareBillingForSelectedChannel` 会按新分组倍率算 `QuotaToPreConsume` 并 `ReserveBilling`：兜底更贵 → 补预留；更便宜 → 结算时退还差额。
- `PriceData.BillingModelName` 会被写成兜底模型，日志的 `model_ratio` / `group_ratio` / `billing_ratio_source` 均反映实际模型与分组。
- `tiered_expr`：`ModelPriceHelper` 会重建 `TieredBillingSnapshot`；切换前必须把旧快照置空。
- 预扣 token 数仍使用原模型的估算（`EstimateRequestToken` 只做一次）；结算按上游真实 usage，差额由 `SettleBilling` 处理。不同 tokenizer 的估算误差可接受，需在文档注明。
- 全部兜底失败：现有 defer 的 `Billing.Refund` 会退回预扣；不要额外手动退款，避免双退。
- 若原模型免费、兜底收费：`info.Billing == nil` 时 `PrepareBillingForSelectedChannel` 会创建会话；反之（原收费、兜底免费）保留会话，结算退到 0。

### 4.5 日志与审计

成功消费日志（`service/log_info_generate.go: GenerateTextOtherInfo`）：

```json
"fallback": {
  "from_model": "gpt-5",
  "from_group": "default",
  "to_model": "claude-sonnet-4-5",
  "to_group": "vip",
  "attempt": 1
}
```

- `fallback` 放在 `other` 顶层（用户可见，用户需要知道自己的请求被兜底）。
- 每次尝试明细放 `other.admin_info.fallback_attempts`：`[{group,model,channel_id,status_code,error}]`，管理员可见；沿用「admin_info 对普通用户整体剥离」的机制。
- 失败请求的错误日志同样写入 `fallback`（是否触发、最终失败原因）；中间尝试的 `processChannelError` 使用 `adminOnly=true`，避免用户日志出现多条噪音。
- `log.model_name` 用实际模型（因为 `relayInfo.OriginModelName` 已切换）；`token_name`/`user` 不变。
- `other.admin_info.use_channel` 已有数组，会自然包含兜底渠道；额外在 attempt 中记录分组，便于排查跨分组。

### 4.6 后端改动文件清单

| 文件 | 改动 |
| --- | --- |
| `model/token.go` | 字段、Get/Set、`Update()` Select 列表 |
| `setting/token_fallback.go`（新） | 类型、常量、归一化校验 |
| `constant/context_key.go` | `ContextKeyTokenFallbackModels` |
| `middleware/auth.go` | 读取兜底配置写上下文 |
| `controller/token.go` | 请求/响应字段、新增/更新校验、`GetTokenFallbackOptions` |
| `router/api-router.go` | 注册 `tokenRoute.GET("/fallback/options", ...)` |
| `relay/common/relay_info.go` | `FallbackState` / `FallbackAttempt` / `RelayInfo.Fallback` |
| `relay/helper/fallback.go`（新） | `ShouldTryFallback`、目标切换辅助、attempt 记录 |
| `controller/relay.go` | 抽 `runRelayAttempt`；主循环后调用兜底阶段 |
| `controller/relay_fallback.go`（新） | 兜底编排：遍历目标、重定价、调用单次尝试、记录 attempt |
| `service/log_info_generate.go` | 写入 `other.fallback` / `admin_info.fallback_attempts` |
| `i18n/locales/zh-CN.yaml`、`i18n/locales/en.yaml` | 后端错误文案（分组不可用、兜底模型重复等） |

---

## 5. 前端实现

### 5.1 密钥编辑（`web/src/features/keys`）

新增组件：

- `components/api-key-fallback-section.tsx`：分区容器 + 启用 Switch + 列表 + 添加按钮 + 计数/说明。
- `components/fallback-model-row.tsx`（或内联）：单行 = 拖拽手柄 + 序号 + 原模型 Combobox（可选，留空=所有模型）+ 分组 Combobox + 模型 Combobox + 上移/下移/删除。
- `lib/fallback.ts`：表单默认值、序列化、去重/上限校验、分组过滤（排除 auto/chain）。

交互细节：

1. 分区标题「兜底模型」，描述「原模型不可用时按顺序切换；可指定原模型（留空=所有模型），按实际模型计费」。
2. 启用开关默认关闭；关闭时列表折叠/禁用，但保留已配置数据（避免误删）。
3. 行内原模型下拉：复用 `mappingOptionsQuery.data.models`（当前 Key 分组可用模型），placeholder「所有模型（可选）」；留空即对任意原模型生效。
4. 行内分组下拉：数据来自 `getUserGroups`，过滤 `auto` 与 `chain:`；显示分组倍率（复用 `ApiKeyGroupCombobox` 的样式与 `GroupRatioBadge`）。
5. 兜底模型下拉：`useQuery(['fallback-models', group], () => getFallbackModels(group))`，按行独立请求、`enabled: !!group`；切换分组立即清空该行模型；加载中 Skeleton，失败可重试。
6. 「添加兜底模型」：在列表末尾追加空行；达到 5 条时禁用并提示上限。
7. 排序：复用 `AutoGroupOrderEditor` 的 drag + 上下箭头模式（motion/react `Reorder`），键盘可操作。
8. 校验（zod）：`fallback_models: z.array(...).max(5)` + `superRefine` 检查空值、同组同模型重复；保存前若分组/模型未选给出 FormMessage。
9. 文案提示：「仅在原模型返回非 2xx 且未产生任何输出 token 时触发；流式响应开始输出后不再切换。」
10. 只读/无权限用户：保留展示，禁用编辑（与现有抽屉一致）。
11. 移动端：行内改为两列（分组/模型），操作按钮放行尾；抽屉底部保存按钮保持现有布局。

改动文件：

| 文件 | 改动 |
| --- | --- |
| `features/keys/types.ts` | `tokenFallbackModelSchema`、`ApiKey`/`ApiKeyFormData` 字段 |
| `features/keys/lib/api-key-form.ts` | schema、默认值、payload 转换、回填 |
| `features/keys/api.ts` | `getFallbackModelOptions(group)` |
| `features/keys/components/api-keys-mutate-drawer.tsx` | 插入 `ApiKeyFallbackSection`，接入 form |
| `features/keys/components/api-key-fallback-section.tsx`（新） | 列表 UI |
| `features/keys/components/api-key-fallback-row.tsx`（新） | 单行 UI |
| `features/keys/lib/fallback.ts`（新） | 纯函数校验/工具 |

### 5.2 使用日志（`web/src/features/usage-logs`）

- `types.ts`：`LogOtherData` 增加 `fallback?: { from_model; from_group?; to_model; to_group?; attempt? }`；`admin_info.fallback_attempts?`。
- `lib/format.ts`：`formatModelName` 返回 `fallback?: {...}`（不依赖管理员权限，用户可见）；`parseLogOther` 已具备。
- 新组件 `components/log-fallback-badge.tsx`：
  - 小号琥珀色徽标：中文「兜」，英文短版「FB」（通过 i18n，避免英文界面出现单个汉字），`aria-label` 为「兜底请求 / Fallback request」；
  - 使用 `Popover`/`HoverCard` 展示：原模型、原分组、兜底模型、兜底分组、第几次尝试、触发原因（管理员可看到状态码/错误）；
  - 键盘可聚焦，点击不触发行点击。
- `components/columns/common-logs-columns.tsx`：模型单元格在 `ModelBadge` 前渲染 `LogFallbackBadge`；若模型本身也有映射，两个徽标并列（顺序：兜 → 模型名 → 映射图标）。
- `components/dialogs/details-dialog.tsx`：新增「兜底链路」区块，列出每次 attempt（管理员含 channel/status/error）。
- `components/usage-logs-mobile-card.tsx`：模型行同步显示「兜」徽标。
- `web/src/i18n/locales/zh.json`、`web/src/i18n/locales/en.json`：新增 `Fallback`、`Fallback request`、`Original model`、`Fallback model`、`Fallback group`、`Attempt {{n}}`、`Billed as {{model}}` 等键。

视觉规范（与现有 `StatusBadge` 一致）：

- 徽标：`bg-warning/12 text-warning`，高度 20px，圆角全，字号 11–12px，内容「兜」；hover 显示完整信息。
- 不新增大面积颜色，不影响现有模型/分组徽标配色。
- 详情弹窗沿用现有 `DetailsDialog` 的分段标题与等宽数值样式。

---

## 6. 关键边界与风险

| 风险 | 处理 |
| --- | --- |
| 流式已输出后无法切换 | `c.Writer.Written()` + `SendResponseCount` 双重判定；已输出直接返回原错误 |
| 兜底目标也无渠道/无价格 | 单条跳过并记录，继续下一条；全部失败返回原错误语义 |
| 重试放大/费用失控 | 最多 5 条，每条仅一次尝试；不做并发竞速；总请求时长自然受客户端超时约束（P1 可加全局 deadline） |
| 计费算成原模型 | 切模型后必须重新 `ModelPriceHelper`，并清空 tiered 快照；回归用例断言 `model_ratio` 与 `log.model_name` |
| 预扣不足/双退款 | 由 `BillingSession` 统一 Reserve/Settle；失败路径只依赖现有 defer Refund |
| 跨分组权限 | 保存时 + 请求时双重校验 `GroupInUserUsableGroups`；禁用 auto/chain 作为兜底目标 |
| auto 分组残留 | 显式覆盖 `auto_group` 上下文并清 index；否则分组倍率取错 |
| 令牌模型限制 | 保存时校验；运行时再查 `token_model_limit`，不通过跳过 |
| 令牌映射/渠道映射叠加 | 令牌映射只作用于客户端原模型；兜底模型仍走渠道映射，语义清晰；文档注明 |
| 免费模型/订阅/赠金 | 复用 `PrepareBillingForSelectedChannel` 的免费与来源逻辑，回归测试覆盖 |
| 并发配置修改 | 请求开始时快照配置到 `RelayInfo.Fallback`，中途修改不影响本次请求 |
| 任务/图片/音频 | P0 明确不支持，前端文案不承诺；后续按 relay 模式扩展 |

---

## 7. 测试计划

### 后端（testify require/assert）

1. `setting/token_fallback_test.go`：空值、超限、重复、`source_model` 可选、非法分组/模型的归一化与错误。
2. `controller/token_test.go`：创建/更新携带兜底配置的持久化与回填；权限失败；`Update()` 白名单包含新字段。
3. `relay/helper/fallback_test.go`：
   - 触发矩阵：503 触发、400 不触发、已写响应不触发、渠道测试不触发、无渠道触发；
   - `source_model` 匹配：指定模型的条目只对该模型生效，空值条目对任意模型生效；
   - 切换目标后 `OriginModelName/UsingGroup/auto_group` 正确；
   - 全部失败返回最后错误并保留 attempt 链。
4. `service/billing` 回归：原模型收费 → 兜底更贵补预留；兜底更便宜结算退款；原免费 → 兜底收费创建会话；全部失败全额退款。
5. 日志生成：`other.fallback` 顶层可见，`admin_info.fallback_attempts` 仅管理员；`model_name` 为实际模型。
6. 集成（可选，httptest + mock channel）：主模型 503 → 兜底成功，断言计费与日志字段。

### 前端（vitest + testing-library）

1. `api-key-fallback-section.test.tsx`：添加/删除/排序/上限、原模型留空=所有模型、切换分组清空模型、重复校验提示。
2. `log-fallback-badge.test.tsx`：有 `fallback` 时渲染「兜」，无则不渲染；Popover 内容；键盘可达。
3. `common-logs-columns` 现有测试补充：模型列同时存在映射/兜底时的顺序与可访问名称。
4. i18n 键完整性（`bun run i18n:sync` 后无缺失）。

### 手工验收

- 新建 Key 配置 3 个兜底 → 保存 → 重新打开回填一致。
- 用测试渠道制造 503/404/超时，确认请求成功走兜底、日志「兜」、费用按兜底模型。
- 流式请求中途断开/已输出：不触发兜底，客户端收到原错误。
- 删除兜底分组权限后再请求：跳过并记录，最终错误可读。

---

## 8. 实施阶段与子代理分工

> 目标：多人并行但**不冲突文件**；每阶段有明确验收门禁。

### P0（MVP，约 2–3 人日）

- 后端：字段/校验/API + relay 兜底阶段 + 计费重定价 + 日志字段。
- 前端：密钥抽屉兜底分区 + 日志「兜」徽标 + 详情链路 + i18n。
- 测试：上述单元/集成用例。

### 子代理拆分

| 子代理 | 职责 | 主要文件 | 依赖 |
| --- | --- | --- | --- |
| A（后端基础） | Token 字段、setting 类型、controller/API、上下文注入、路由 | `model/token.go`、`setting/token_fallback.go`、`constant/context_key.go`、`middleware/auth.go`、`controller/token.go`、`router/api-router.go` | 无 |
| B（relay 核心） | `runRelayAttempt` 抽取、兜底编排、触发判定、目标切换 | `controller/relay.go`、`controller/relay_fallback.go`、`relay/helper/fallback.go`、`relay/common/relay_info.go` | A 的字段/上下文 |
| C（计费与日志） | 重定价接入、预留/结算回归、`other.fallback` 与 admin 明细 | `service/log_info_generate.go`、计费相关测试（尽量不改 `billing.go`） | B 的 FallbackState |
| D（密钥前端） | 抽屉分区、行组件、表单校验、模型下拉请求 | `web/src/features/keys/**`、keys i18n | A 的 API 契约（可先用 mock） |
| E（日志前端） | 「兜」徽标、Popover、详情、移动端、i18n | `web/src/features/usage-logs/**`、usage i18n | C 的日志字段（可先用 mock） |
| F（QA/联调） | 触发矩阵、计费对账、流式/权限回归、预览核对 | 测试文件、`docs/fallback-models/**` | A–E 完成后 |

并行策略：

1. **第 1 天**：A 先落字段与 API 契约（30 分钟内给 D 明确 JSON）；D/E 用 mock 数据先行开发 UI；B 基于契约实现编排；C 准备计费测试。
2. **第 2 天**：B+C 联调计费与日志；D+E 接真实接口；F 开始跑触发矩阵。
3. **第 3 天**：全量回归（SQLite/MySQL/PG 各跑一次迁移与关键用例）、流式手工验证、文档与预览更新。

**文件所有权**：A/B/C/D/E 各自修改上表文件，跨文件改动（如 `relay_info.go`）由 B 统一提交，C 只加字段读取；避免并发编辑同一文件。  
**合并顺序**：A → B → C → D/E 并行 → F。  
**门禁**：每个 PR 必须通过 `go test ./...`（或受影响包）、`bun run build`、`bun run lint`、i18n sync check。

### P1（可选增强）

- 支持图片/音频/任务类兜底（按 relay mode 扩展）；每目标多次渠道重试；全局兜底超时；统计「兜底成功率」面板；兜底触发条件可配置（429 是否兜底）；按模型权重/优先级的高级路由。

---

## 9. 验收标准（Definition of Done）

1. 密钥编辑页可配置 0–5 个「原模型（可选）+ 分组 + 模型」兜底项；同一原模型可配多条，支持排序、删除、重复校验、权限校验。
2. 模型下拉确实按所选分组自动加载，切换分组会清空并重新加载。
3. 原模型 503/404/超时/无渠道时，按顺序尝试兜底并成功返回；已输出流式响应不切换。
4. 计费按最终实际模型与分组计算，账单/日志倍率一致；全部失败不产生费用。
5. `/usage-logs/common` 模型列显示「兜」徽标，悬浮/详情能看到原模型、兜底模型、分组与尝试次数。
6. 普通用户只能看到自己的兜底徽标信息，管理员能看到每次失败渠道/状态码/错误。
7. SQLite、MySQL、PostgreSQL 迁移通过；旧 Key 行为不变。
8. 单元/集成/前端测试通过，i18n 无缺失，预览图与实现一致。

---

## 9.1 实现备注（P0 落地时确认的边界）

- 触发判定同时要求：未向下游写出任何字节（`Writer.Written()`）、`SendResponseCount == 0`、`ReceivedResponseCount == 0`、请求上下文未取消、非渠道测试、relay 格式在白名单内。
- 客户端/策略类错误码（invalid_request、access_denied、sensitive_words_detected、insufficient_user_quota、pre_consume_token_quota_failed、prompt_blocked）无条件不触发兜底，即使其状态码落在 5xx。
- 兜底目标分组必须是具体分组：`auto` 与 `chain:*` 在保存时与请求时都会被拒绝。
- 透传（pass-through）渠道会原样重放客户端请求体，因此切换兜底目标时会同时改写缓存请求体中的 `model` 字段；无 `model` 字段的请求体保持原样。
- `PrepareFallbackTarget` 在赋值 `UpstreamModelName`/`IsModelMapped` 之前先补齐 `ChannelMeta`，避免「原分组无可用渠道」首次尝试就失败时的空指针。
- 一次兜底尝试一旦已经产生输出（收到或已下发），立即停止后续兜底，避免响应拼接与不可见的上游计费。

---

## 10. 预览文件

- HTML：`docs/fallback-models/preview.html`
  - 密钥编辑：`?view=keys`（或 `#keys`）
  - 使用日志：`?view=logs`（或 `#logs`）
- PNG（含标注）：`docs/fallback-models/preview-keys.png`、`docs/fallback-models/preview-logs.png`
- PNG（纯净版）：`docs/fallback-models/preview-keys-clean.png`、`docs/fallback-models/preview-logs-clean.png`
- HTML 支持 `&anno=0` 隐藏标注卡，`&shot=1` 隐藏切换按钮（截图用）
