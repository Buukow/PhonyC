# PhonyG — AI API 轻量中转网关 MVP 需求与设计

- **日期**: 2026-07-29
- **状态**: 已评审（brainstorming §1–§5 用户确认）
- **实现取向**: 方案 2 — 配置驱动插件式渠道
- **仓库**: PhonyG
- **视觉**: `STYLEKIT_STYLE_REFERENCE_warm-sage-admin.md`（暖米青绿后台）

---

## 1. 产品概述

### 1.1 一句话

PhonyG 是一个用 Go 实现的**轻量 AI API 中转网关**：第三方客户端先打到本服务，本服务在 **HTTP 层修改/重组 Header**，将 **Body 以原始字节穿透** 到上游（官方 API 或其它聚合中转），并提供单机管理台维护渠道、模型映射、用户 Key 与客户端伪装预设。

### 1.2 核心设计原则

| # | 原则 | 说明 |
|---|------|------|
| P1 | Header 为王 | 鉴权替换、Host、客户端伪装只在 Header 管线完成 |
| P2 | Body 默认玻璃穿透 | 不整包反序列化、不 re-marshal；客户端何种 API 形态就原样转发 |
| P3 | 最小例外：model | 为路由可 **peek** 顶层 `model`；仅当映射开启改写时做**字节级**替换 |
| P4 | 流式原生 | 请求侧可有限缓冲 JSON；响应 SSE/非流式透传并及时 Flush |
| P5 | 配置驱动渠道 | 渠道 = 协议插件 + base_url + key + 额外 Header 模板 + 模型表 |
| P6 | 伪装绑在用户 Key | 同一上游可因不同 Key 呈现不同客户端指纹 |
| P7 | 单机可信 | SQLite、单管理员、用户 Key 可后台明文查看 |

### 1.3 目标用户与部署

- 单机自用 / 小范围分发多个用户 API Key
- 单管理员；SQLite 文件库
- 单进程（可 Docker）；数据目录持久化

### 1.4 参考素材（非实现绑定）

仓库内 New API 风格抓包仅作 Codex / Claude Code **伪装预设** 与联调对照：

- `relay-user-request.log` — 客户端 → 中转
- `relay-channel-request.log` — 中转 → 上游
- `relay-channel-response.log` — 上游 → 中转
- `relay-combined.log` — 合并视图

观测到的客户端形态包括：

- **Codex TUI** `codex-tui/0.145.0` → `POST /v1/responses`，特征头：`Originator`、`X-Codex-*`、`Accept: text/event-stream`
- **Claude Code** `claude-cli/2.1.220` → `POST /v1/messages`，特征头：`Anthropic-Version`、`Anthropic-Beta`、`X-App`、`X-Stainless-*`
- **普通 SDK** → `POST /v1/chat/completions` 等极简头

**path → 协议 的固定映射（参考 New API `relay-router.go`）**：请求 path 本身即决定 API 形态，从而决定应路由到的协议渠道。

| path | API 形态 | 目标协议 |
|------|----------|----------|
| `POST /v1/messages` | Claude Messages | `anthropic` |
| `POST /v1/responses` | OpenAI Responses | `openai` |
| `POST /v1/chat/completions` | OpenAI Chat | `openai` |
| `POST /v1/completions` | OpenAI 文本补全 | `openai` |
| `GET /v1/models` | 模型列出 | 由 Header 判别（见 §5.8） |

---

## 2. 范围

### 2.1 MVP 必须

- Go + Gin 代理入口；`httputil.ReverseProxy`（或等价）+ Header 修改
- 双协议渠道插件：`openai`、`anthropic`；**接入渠道时显式选择协议**
- 路由：**先按请求 path 判定协议**（`/v1/messages`→anthropic，`/v1/responses` `/v1/chat/completions` `/v1/completions`→openai），**再按模型 ID 在该协议渠道中选渠**
- 多渠道；渠道级 `priority`；模型命中后高优先级优先，同优先级 **随机**（不禁止随机分流）
- 模型映射：`client_model` / `upstream_model` / 每映射 `rewrite_model` 开关（双模式 C）
- **模型列出接口**：`GET /v1/models`（及 anthropic 形态）聚合返回当前所有 enabled 渠道配置的模型，New API 兼容格式
- 用户 API Key：启禁、明文可展示、伪装三模式
- 内置 + 可编辑客户端预设（Codex / Claude Code 种子）
- 管理端：首次强制创建管理员；JWT 登录；改密
- 可观测：元数据日志、按 Key 统计、简单仪表盘
- URL：`/v1/*` 代理，`/api/*` 管理，`/` 前端
- 前端：React 18 + Vite 5 + Tailwind + shadcn/ui，暖米青绿

### 2.2 明确非目标（MVP）

- 跨渠道主备 failover、同渠多 Key 重试
- OpenAI ↔ Anthropic body 互译
- 成功请求 body/全量 header 落库
- 精确字节计量（`bytes_in`/`bytes_out`）：与玻璃穿透/SSE 透传冲突，MVP 不做
- 多管理员、RBAC、计费充值、注册多租户
- QPS 限流、IP 白名单、Webhook
- 多实例配置中心、响应内容审查
- 除 `/v1/models` 之外的 body 聚合／协议互译（models 列出属 MVP 必须，见 §2.1）

### 2.3 后期

1. 模型级主备渠道  
2. Key 池 + 对 429/5xx 换 Key 重试  
3. 按 path 的伪装、动态 Header  
4. 配额硬拒绝  
5. 配置导入（如部分 New API 兼容）

---

## 3. 总体架构

```text
客户端 (Codex / Claude Code / SDK)
        │  Authorization: Bearer <user_api_key>
        ▼
┌──────────────────────────────────────────────┐
│  PhonyG 单二进制                              │
│  Gin                                          │
│  /          → 管理前端 dist                    │
│  /api/*     → 管理 REST (管理员 JWT)           │
│  /v1/*      → 代理 Pipeline                    │
│                                               │
│  Pipeline                                     │
│   1. 校验 user key + 伪装策略                   │
│   2. Path → 协议 (responses/chat→openai;        │
│           messages→anthropic)                  │
│   3. Peek body.model                           │
│   4. Router: 协议内 模型匹配 + priority/随机     │
│   5. Protocol Plugin (openai | anthropic)      │
│   6. Header 模板：渠道 extra + 伪装            │
│   7. 可选字节级 rewrite model                  │
│   8. ReverseProxy 透传                         │
│   9. request_meta + 统计                       │
│                                               │
│  SQLite + 内存热缓存（写后失效）                │
└──────────────────────────────────────────────┘
        ▼
   上游 base_url
```

### 3.1 逻辑包边界

| 包 | 职责 |
|----|------|
| `proxy` | Gin 挂载、ReverseProxy、Flush、超时 |
| `pipeline` | 认证、peek/rewrite、编排 |
| `protocol` | openai/anthropic 插件接口 |
| `template` | Header 模板与预设应用 |
| `router` | 协议内 模型 → 渠道选择 |
| `modelcatalog` | 聚合各渠道模型，供 `/v1/models` 列出 |
| `store` | SQLite 与仓储 |
| `adminapi` | 管理 REST |
| `web` | 前端工程 |
| `metrics` / `logmeta` | 统计与元数据日志 |

### 3.2 配置热更新（最小方案）

单机单进程，采用「全局版本号 + 全量重建」的最小一致性方案，不做细粒度失效：

- 内存中维护一份只读快照 `configSnapshot{channels, channelModels, userKeys, presets, catalogByProtocol}` 与一个 `version uint64`。
- 代理热路径通过 `atomic.Pointer[configSnapshot]`（或 `RWMutex` 保护的指针）**只读**当前快照；一次请求内只加载一次，保证请求内自洽。
- 任一管理写 API 成功提交 SQLite 后：重新从库全量加载、构建新快照、`version+1`、原子替换指针。旧快照被仍在处理的在途请求继续持有，无需加锁等待。
- 无需区分「相关缓存」；重建成本对单机配置量（渠道/模型/Key 皆百级）可忽略。

约束：写操作串行化（管理端天然低并发），避免两次写之间的丢失更新。

---

## 4. 数据模型（SQLite）

### 4.1 `admin_user`

- `id`, `username`, `password_hash`, `created_at`, `updated_at`
- 无记录则进入强制 Setup

### 4.2 `channels`

- `id`, `name`, `enabled`
- `protocol`: `openai` | `anthropic` — **接入渠道时即选定**；决定该渠可承接的入口 path 族与鉴权骨架（见 §5.2 路由）
- `base_url`
- `api_key`（MVP 明文存储，依赖主机与卷权限；文档警示）
- `priority`（整数，越大越优先）
- `extra_headers_json`
- `timeout_ms`
- `created_at`, `updated_at`

MVP：**一渠一上游 Key**。

### 4.3 `channel_models`

- `id`, `channel_id`
- `client_model` — 客户端 body 中的模型名
- `upstream_model` — 改写目标名
- `rewrite_model` — bool
- `enabled`
- Unique `(channel_id, client_model)`

**选渠算法（path → 协议 → 模型 ID → 优先级）**

1. **由入口 path 定协议**（见 §5.2 映射表）：`/v1/messages` → `anthropic`；`/v1/responses`、`/v1/chat/completions`、`/v1/completions` 等 → `openai`。得到 `required_protocol`。
2. Peek 顶层 `model` → `client_model`。
3. 候选集 = enabled channel ∩ `channel.protocol == required_protocol` ∩ 存在 enabled 的 `channel_models` 命中 `client_model`。
4. 按 `channel.priority` 降序分档；**最高档内随机**（同优先级随机为既定行为，不禁止）。
5. 选中渠对应的那条 mapping 决定 `rewrite_model` / `upstream_model`；若 `rewrite_model` 为真：字节替换 model value + 修正 `Content-Length`（规则见 §5.3）。
6. 无候选：网关错误 `model_not_found`（非上游透传）；协议不匹配（如 anthropic 渠命中但 path 为 `/v1/responses`）不进入候选，等同无匹配。

**关于 rewrite 混配的非确定性**：同一 `client_model` 在同优先级的多个渠中，若各条 mapping 的 `rewrite_model`/`upstream_model` 不一致，则随机选渠会导致同一请求被随机改写成不同上游 body。这是**允许的既定行为**（同优先级随机的自然结果），运维需自行保证同档渠的改写语义一致；MVP 不做一致性校验，仅在管理台对「同 client_model、同优先级、rewrite 目标不一致」给出**非阻断提示**。

### 4.4 `user_keys`

- `id`, `name`, `key`（明文可展示）, `enabled`, `remark`
- `impersonation_mode`: `passthrough` | `preset` | `custom`
- `preset_id` nullable
- `custom_headers_json`
- `created_at`, `updated_at`

### 4.5 `client_presets`

- `id`, `name`, `description`, `version_label`
- `headers_json`（可含 `{{version}}` 等占位）
- 可选 `remove_headers` 列表（JSON 内或并列字段）
- `builtin` bool

### 4.6 `request_meta`

- `id`, `request_id`, `created_at`
- `user_key_id`, `client_model`, `upstream_model`, `channel_id`
- `method`, `path`, `status_code`
- `ttfb_ms`, `total_ms`
- `error_summary`（短文本）
- `impersonation_mode`
- **不存** 成功 body；**不存** 全量 header

### 4.7 `key_stats_daily`

- `user_key_id`, `day`, `requests`, `errors`
- Unique `(user_key_id, day)`
- **MVP 不做** `bytes_in`/`bytes_out`：玻璃穿透 + SSE 流式下无法在不破坏透传的前提下精确计量，明确降为非目标（见 §2.2）。后期若做，用请求 `Content-Length` + 响应累计写入字节做**近似**，并在字段上标注 approx。

### 4.8 `app_settings`

- KV：如 `log_retention_days`

---

## 5. 代理数据面

### 5.1 生命周期

```text
POST /v1/... + Bearer user_key
 → AuthUserKey（401 if bad/disabled）
 → 有限缓冲请求体（上限可配，默认建议 32–64MB）
 → PeekModel
 → RouteChannel
 → MaybeRewriteModel
 → BuildUpstreamHeaders (plugin + channel + impersonation)
 → ReverseProxy
 → 记 meta + stats
 → 上游状态/body 原样返回（网关错误除外）
```

### 5.2 URL 与协议路由

**path → 协议映射（进入代理时先判定，参考 New API）**

| 客户端 path | API 形态 | 要求渠道 `protocol` |
|-------------|----------|---------------------|
| `POST /v1/chat/completions`、`/v1/completions` | OpenAI Chat | `openai` |
| `POST /v1/responses`、`/v1/responses/compact` | OpenAI Responses | `openai` |
| `POST /v1/messages` | Anthropic Messages | `anthropic` |
| `GET /v1/models`、`/v1/models/:id` | 模型列出 | 见 §5.8（按 Header 判定返回格式） |

- 选渠时**只在与该 path 协议匹配的渠道中**筛选（详见 §4.3）。
- `upstream_url = channel.base_url + request.URL.Path + ?query`
- Path/Query **原样**；不做协议 path 翻译（openai↔anthropic body 互译仍是非目标）
- MVP 不在文档表格内的 `POST /v1/*` path：透传到命中渠道，但**不做协议校验**，按 openai 形态 peek model；无法 peek 则 §5.3 处理

### 5.3 Body 规则

| 情况 | 行为 |
|------|------|
| 默认 / rewrite off | 原始字节转发；只读 peek |
| rewrite on | 只替换顶层 `"model"` 的字符串值；禁止整包 Unmarshal/Marshal（严格规则见下） |
| 非 JSON 或无法 peek 顶层 model | 无法选渠 → 网关 400（`missing_model`） |
| `GET /v1/models` 等模型列出 path | 不选渠、不读 body；由 §5.8 处理 |
| 其它无 body / 无顶层 model 的 `POST /v1/*` | 网关 400（`missing_model`）。MVP 不实现「无 model 透传转发」 |
| 超 body 上限 | 网关 413（`request_too_large`） |

**顶层 model 的 peek / rewrite 严格规则**

- **只认顶层键**：使用流式 JSON 扫描（如 `json.Decoder` token 流或等价状态机）定位深度为 1 的 `"model"` 键，**不得**匹配嵌套对象内的 `model`（如 `{"metadata":{"model":...}}` 不算）。
- **只处理字符串值**：顶层 `model` 值必须是 JSON 字符串；非字符串（数组/对象/数字）视为无法 peek → 400。
- **保留原始编码**：peek 时对值做 JSON 反转义得到逻辑模型名用于路由匹配；rewrite 时把 `upstream_model` 按 JSON 字符串规则重新转义后原位替换该值的字节区间，其余字节一律不动。
- **Content-Length**：rewrite 后按新字节长度重设 `Content-Length`。
- **Transfer-Encoding: chunked**：MVP 请求侧整包缓冲后以固定 `Content-Length` 转发（去除 `Transfer-Encoding`）；不透传 chunked 请求体。
- **多次出现**：只替换第一个顶层 `model`；正常客户端不会有重复顶层键。

**流式假设（MVP）**：常见客户端为整包 JSON 请求 + 可选 SSE 响应。响应不完整缓冲；请求可整包缓冲。

### 5.4 Header 合并顺序

1. 去掉 hop-by-hop；用户 `Authorization` 不转发  
2. 重设 `Host`  
3. 协议插件鉴权骨架  
   - openai: `Authorization: Bearer {{api_key}}`  
   - anthropic: `x-api-key: {{api_key}}`；缺省可补 `anthropic-version`（默认 `2023-06-01`）  
4. 渠道 `extra_headers`（模板渲染）  
5. 伪装（**鉴权头受保护**：无论哪种模式，步骤 3 写入的协议鉴权头和步骤 4 的渠道 extra 都不被伪装逻辑覆盖或剥离）：  
   - **passthrough**：保留客户端业务头；协议鉴权已在步骤 3 覆盖客户端原鉴权  
   - **preset / custom**：对「客户端业务头」采取 **strip-then-apply**——先丢弃全部客户端业务头（hop-by-hop 与用户鉴权在步骤 1 已剥离），再套用预设/自定义模板；模板**不得**声明协议鉴权头（如 `Authorization`/`x-api-key`），此类头始终由步骤 3 决定。需要保留个别客户端头时，在模板中显式声明或改用 passthrough  

   **顺序保证**：步骤 3 协议鉴权 → 步骤 4 渠道 extra → 步骤 5 伪装 strip/apply（只作用于业务头），因此伪装永远在鉴权之后，且不触碰鉴权与 extra。
6. 若 rewrite：确保 `Content-Length` 正确  
7. MVP 建议对上游去掉客户端 `Accept-Encoding` 或强制 identity，降低压缩陷阱  

### 5.5 响应

- 默认不改 body  
- SSE 及时 Flush；`/v1` 避免缓冲中间件  
- 上游 4xx/5xx 原样回传  
- 网关错误统一 JSON 结构：

```json
{
  "error": {
    "message": "model not found",
    "type": "gateway_error",
    "code": "model_not_found"
  }
}
```

**网关错误码表**（`type` 恒为 `gateway_error`，与上游透传错误区分；上游 4xx/5xx 原样回传不套此结构）：

| HTTP | `code` | 触发场景 |
|------|--------|----------|
| 401 | `invalid_api_key` | user key 缺失、无效或已禁用 |
| 400 | `model_required` | 需选渠路径但 body 顶层读不到 `model` |
| 400 | `invalid_request_body` | body 非 JSON 或无法 peek |
| 404 | `model_not_found` | 有 model 但无 enabled 的 `(protocol, client_model)` 命中 |
| 409 | `protocol_mismatch` | path 推断的协议与命中渠道 `protocol` 不一致（详见 §5.2） |
| 413 | `body_too_large` | 请求体超过 `PHONYG_MAX_BODY_BYTES` |
| 502 | `upstream_unreachable` | 连接上游失败 / DNS / 拒绝 |
| 504 | `upstream_timeout` | 超过渠道 `timeout_ms` |

- 响应头可加 `X-Request-Id`（回显或生成，用于日志关联）

### 5.6 超时与取消

- 使用渠道 `timeout_ms`  
- 客户端断开则 cancel 上游

### 5.7 内置预设种子（可编辑）

**codex-tui**（源自日志，值可调）

- `User-Agent`: `codex-tui/{{version}} (Debian 12.0.0; x86_64) xterm-256color (codex-tui; {{version}})`
- `Originator`: `codex-tui`
- `Accept`: `text/event-stream`
- 可选静态 `X-Codex-Beta-Features` 等
- 会话类头：若客户端已有则保留，缺失可生成 UUID（实现时按此默认）

**claude-cli**

- `User-Agent`: `claude-cli/{{version}} (external, cli)`
- `X-App`: `cli`
- `Anthropic-Version`: `2023-06-01`
- 默认 `Anthropic-Beta` 长串（与抓包对齐，可在管理台改）
- `X-Stainless-*` 骨架字段

完整键值以仓库根目录抓包日志为 **seed 源**（`relay-user-request.log` 中 Codex/Claude 条目的 `user_headers`），导入后可在管理台编辑；PRD 不要求与抓包永久逐字节锁定。

**占位符解析规则**

- 语法 `{{var}}`；变量来源限定为 preset 自身字段，MVP 仅支持 `{{version}}`（取 `client_presets.version_label`）。
- 解析时机：构建上游 Header 时一次性渲染。
- 未定义变量：渲染为空串并记一条 warning，不阻断请求（避免因模板笔误导致整条链路 500）。
- `custom` 模式的 `custom_headers_json` 同样走此渲染；其可用变量集合与 preset 一致。

### 5.8 模型列出（`GET /v1/models`）

参考 New API：**用 Header 判别客户端期望的协议格式**，再聚合返回该用户 Key 可用的模型集合。

**协议判别**（进入 `/v1/models` 时）：

| 条件 | 返回格式 |
|------|----------|
| 同时存在 `x-api-key` 与 `anthropic-version` | Anthropic 列出格式 |
| 其它（默认） | OpenAI 列出格式 |

**模型来源**：聚合所有 `enabled` 渠道下 `enabled` 的 `channel_models.client_model`，按 `client_model` 去重（跨渠道同名只出现一次）。MVP 不按 Key 做模型白名单过滤（无此字段），返回全量可用模型。

**OpenAI 格式**（默认）：

```json
{
  "object": "list",
  "data": [
    { "id": "Grok-4.5", "object": "model", "created": 1626777600, "owned_by": "phonyg" }
  ]
}
```

**Anthropic 格式**（命中上表第一行）：

```json
{
  "data": [
    { "type": "model", "id": "Grok-4.5", "display_name": "Grok-4.5", "created_at": "2021-07-20T00:00:00Z" }
  ],
  "first_id": "Grok-4.5",
  "last_id": "Grok-4.5",
  "has_more": false
}
```

- `GET /v1/models/:id`：按 id 精确返回单条（同上格式的单对象），未命中返回 404 `model_not_found`。
- 该 path **不选渠、不读 body、不打上游**；直接由 `modelcatalog` 从内存快照生成。

---

## 6. 管理 API

### 6.1 引导与认证

| 端点 | 说明 |
|------|------|
| `GET /api/setup/status` | `{ initialized: bool }` |
| `POST /api/setup` | 仅未初始化：创建管理员 |
| `POST /api/auth/login` | 返回 JWT |
| `POST /api/auth/change-password` | 改密 |
| `PATCH /api/auth/profile` | 可选改用户名 |

除 setup/login/health 外，`/api/*` 需管理员 JWT。  
`/v1/*` 仅用户 API Key。

### 6.2 资源 CRUD

- 渠道：`/api/channels`、`/api/channels/:id`
- 模型：`/api/channels/:id/models`、`/api/channel-models/:id`
- Key：`/api/keys`、`/api/keys/:id`（明文 key 返回；前端遮罩+显示/复制）
- 预设：`/api/presets`、`/api/presets/:id`
- 日志：`GET /api/logs` 分页筛选
- 仪表盘：`GET /api/dashboard/summary` — 建议字段：`requests_today`, `errors_today`, `requests_7d`, `error_rate_7d`, `top_keys[]`, `top_models[]`, `recent_errors[]`
- Key 统计：`GET /api/keys/:id/stats?range=7d|30d` — 按日 `requests`/`errors` 序列
- JWT：HS256；claims 含 `sub`=admin id、`username`；默认 TTL **24h**（可配置）；不实现 refresh token（MVP 重新登录）
- 设置：`GET/PATCH /api/settings`
- 健康：`GET /api/health`（可公开）

所有写成功后失效代理缓存。

---

## 7. 前端信息架构

**栈**: React 18、Vite 5、Tailwind CSS、shadcn/ui、lucide 细线图标  
**风格**: 暖米白 canvas `#faf8f5`、主色 `#4a9d9a`、白卡片 `rounded-2xl` 轻阴影；见仓库 Stylekit

| 路由 | 页面 |
|------|------|
| `/setup` | 首次初始化 |
| `/login` | 登录 |
| `/` | 仪表盘 |
| `/channels` | 渠道列表 |
| `/channels/:id` | 渠道详情 + 模型映射表 |
| `/keys` | 用户 Key（伪装策略） |
| `/presets` | 客户端预设 |
| `/logs` | 请求元数据日志 |
| `/settings` | 改密与系统设置 |

布局：可折叠侧栏约 `w-60`、顶栏 sticky 半透明、主区 `p-6/8`。

开发期 Vite 代理 `/api` 与 `/v1`；生产 `dist` 由 Gin 托管或 embed。

---

## 8. 部署

环境变量（名称可在实现时微调，语义固定）：

| 变量 | 含义 |
|------|------|
| `PHONYG_ADDR` | 监听地址，如 `:8080` |
| `PHONYG_DATA_DIR` | 数据目录（SQLite 等） |
| `PHONYG_JWT_SECRET` | 可缺省：首次自动生成并持久化到 data |
| `PHONYG_MAX_BODY_BYTES` | 请求体上限 |

产物：单二进制 + `data/`；可选 Docker 挂卷。

---

## 9. 验收标准

1. 普通 SDK：`/v1/chat/completions` 非流式往返成功  
2. 流式 SSE（responses 或 messages）可完整消费，无错误拼包  
3. `rewrite_model` 开关对比：off 时 body 与客户端一致；on 时仅 model 值变为 `upstream_model` 且 Content-Length 正确  
4. 两渠同模型：高 priority 稳定中选；同 priority 多次请求出现分流  
5. Key 禁用 → 401；三种伪装模式在 mock 上游可观察 Header 差异  
6. 首次 Setup → 登录 → 渠道/模型/Key/预设 CRUD → 仪表盘与日志可见  
7. 成功路径不写 body 到 `request_meta`

---

## 10. 风险

| 风险 | 缓解 |
|------|------|
| 虚拟模型 vs 不碰 body | 双模式 + 字节手术刀 |
| SSE 被缓冲 | 专用代理路径、Flush、测试 |
| 预设随官方客户端过时 | 可编辑预设 + version |
| 明文密钥 | 单机模型文档警示；后期 at-rest 加密 |
| 大 body OOM | 上限与拒绝 |
| 插件过度设计 | MVP 仅两协议 + 通用 Header 模板 |

---

## 11. 决策记录（brainstorming）

| 主题 | 决策 |
|------|------|
| 模型改写 | C 双模式 |
| 部署 | 单机 SQLite；单管理员；多用户 Key |
| 负载 | MVP=A 一模型一渠一 key 失败原样；后期主备 C |
| 伪装 | 按 Key：默认/预设/自定义 |
| 可观测 | B（元数据日志+统计+仪表盘） |
| 前端 | React18/Vite5/Tailwind/shadcn + 暖米青绿 |
| 协议 | B openai + anthropic |
| 选渠 | 有模型则 priority，并列随机；peek body.model |
| Key 字段 | 基础+明文可显+启禁 |
| 日志深度 | A 仅元数据 |
| 管理员 | 首次强制设置，设置页改密 |
| URL | C：`/v1` + `/api` + `/` |
| 架构方案 | **方案 2 配置驱动插件式渠道** |

---

## 12. 实现注意（给后续 plan）

- 禁止对成功路径 body 做 `json.Unmarshal` → 修改 → `Marshal` 全包回写  
- 代理与管理中间件分离，避免全局 Logger 读尽 body  
- 内置预设 migration/seed 与 Stylekit 一并纳入首期交付  
- 抓包中的真实 Bearer 仅出现在参考日志，**不得**写入代码或文档示例为可用密钥；文档示例用占位符  

---

**文档结束。** 用户确认本文件后，再进入 implementation plan 阶段。
