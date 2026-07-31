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

- **Codex CLI** `codex_exec/0.145.0` → `POST /v1/responses`，特征头：`Originator`、`X-Codex-*`、`Accept: text/event-stream`
- **Claude Code** `claude-cli/2.1.220` → `POST /v1/messages`，特征头：`Anthropic-Version`、`Anthropic-Beta`、`X-App`、`X-Stainless-*`
- **普通 SDK** → `POST /v1/chat/completions` 等极简头

---

## 2. 范围

### 2.1 MVP 必须

- Go + Gin 代理入口；`httputil.ReverseProxy`（或等价）+ Header 修改
- 双协议渠道插件：`openai`、`anthropic`
- 多渠道；渠道级 `priority`；模型命中后高优先级优先，同优先级 **随机**
- 模型映射：`client_model` / `upstream_model` / 每映射 `rewrite_model` 开关（双模式 C）
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
- 多管理员、RBAC、计费充值、注册多租户
- QPS 限流、IP 白名单、Webhook
- 多实例配置中心、响应内容审查

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
│   2. Peek body.model                           │
│   3. Router: 模型匹配 + priority / random      │
│   4. Protocol Plugin (openai | anthropic)      │
│   5. Header 模板：渠道 extra + 伪装            │
│   6. 可选字节级 rewrite model                  │
│   7. ReverseProxy 透传                         │
│   8. request_meta + 统计                       │
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
| `router` | 模型 → 渠道选择 |
| `store` | SQLite 与仓储 |
| `adminapi` | 管理 REST |
| `web` | 前端工程 |
| `metrics` / `logmeta` | 统计与元数据日志 |

### 3.2 配置热更新

管理写 API 成功提交 SQLite 后 bump 版本 / 清空相关内存缓存；代理热路径只读缓存。

---

## 4. 数据模型（SQLite）

### 4.1 `admin_user`

- `id`, `username`, `password_hash`, `created_at`, `updated_at`
- 无记录则进入强制 Setup

### 4.2 `channels`

- `id`, `name`, `enabled`
- `protocol`: `openai` | `anthropic`
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

**选渠算法**

1. Peek 顶层 `model` → `client_model`
2. 过滤 enabled 的 mapping 与 channel
3. 按 channel.priority 降序；最高档内 random
4. 若 `rewrite_model`：字节替换 model value + 修正 `Content-Length`
5. 无匹配：网关错误（非上游透传）

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
- 可选 `bytes_in`/`bytes_out`（难以准确时可先 0）

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

### 5.2 URL

- `upstream_url = channel.base_url + request.URL.Path + ?query`
- Path/Query **原样**；不做协议 path 翻译

### 5.3 Body 规则

| 情况 | 行为 |
|------|------|
| 默认 / rewrite off | 原始字节转发；只读 peek |
| rewrite on | 只替换顶层 `"model"` 的字符串值；禁止整包 Unmarshal/Marshal |
| 非 JSON 或无法 peek | 无法选渠 → 网关 400 |
| 无 body / 非 POST（如部分 GET） | **MVP：凡进入需选渠的代理路径，必须能从 JSON body 顶层读到 model**；否则网关 400。不在 MVP 实现「无 model 的透传转发」或 models 列表聚合 |
| 超 body 上限 | 网关 413/400 |

**流式假设（MVP）**：常见客户端为整包 JSON 请求 + 可选 SSE 响应。响应不完整缓冲；请求可整包缓冲。

### 5.4 Header 合并顺序

1. 去掉 hop-by-hop；用户 `Authorization` 不转发  
2. 重设 `Host`  
3. 协议插件鉴权骨架  
   - openai: `Authorization: Bearer {{api_key}}`  
   - anthropic: `x-api-key: {{api_key}}`；缺省可补 `anthropic-version`（默认 `2023-06-01`）  
4. 渠道 `extra_headers`（模板渲染）  
5. 伪装：  
   - **passthrough**：保留客户端业务头，再盖鉴权  
   - **preset / custom**：**先剥离** hop-by-hop 与用户鉴权后，对「客户端剩余业务头」采取 **strip-then-apply**：丢弃客户端业务头，仅应用预设/自定义模板（再写协议鉴权与渠道 extra）。需要保留个别客户端头时，在模板中显式声明或改用 passthrough  
6. 若 rewrite：确保 `Content-Length` 正确  
7. MVP 建议对上游去掉客户端 `Accept-Encoding` 或强制 identity，降低压缩陷阱  

### 5.5 响应

- 默认不改 body  
- SSE 及时 Flush；`/v1` 避免缓冲中间件  
- 上游 4xx/5xx 原样回传  
- 网关错误统一 JSON，例如：

```json
{
  "error": {
    "message": "model not found",
    "type": "gateway_error",
    "code": "model_not_found"
  }
}
```

- 响应头可加 `X-Request-Id`

### 5.6 超时与取消

- 使用渠道 `timeout_ms`  
- 客户端断开则 cancel 上游

### 5.7 内置预设种子（可编辑）

**codex-tui**（源自 Codex CLI 0.145.0 日志，值可调）

- `User-Agent`: `codex_exec/{{version}} (Debian 12.0.0; x86_64) dumb (codex_exec; {{version}})`
- `Originator`: `codex_exec`
- `Accept`: `text/event-stream`
- 可选静态 `X-Codex-Beta-Features` 等
- 会话类头：若客户端已有则保留，缺失可生成 UUID（实现时按此默认）

**claude-cli**

- `User-Agent`: `claude-cli/{{version}} (external, sdk-cli)`
- `X-App`: `cli`
- `Anthropic-Version`: `2023-06-01`
- 默认 `Anthropic-Beta` 长串（与抓包对齐，可在管理台改）
- `X-Stainless-*` 骨架字段

完整键值以仓库根目录抓包日志为 **seed 源**（`relay-user-request.log` 中 Codex/Claude 条目的 `user_headers`），导入后可在管理台编辑；PRD 不要求与抓包永久逐字节锁定。

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
