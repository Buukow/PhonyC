---
layout: default
title: API 路径与协议
parent: 配置参考
nav_order: 2
permalink: /reference/api/
---

# API 路径与协议

## 代理路径

| 方法与路径 | 请求协议 | 渠道协议 | 说明 |
|---|---|---|---|
| `POST /v1/chat/completions` | OpenAI | `openai` | Chat Completions，支持流式响应透传。 |
| `POST /v1/completions` | OpenAI | `openai` | Legacy Completions。 |
| `POST /v1/responses` | OpenAI | `openai` | Responses API，适合 Codex 客户端。 |
| `POST /v1/messages` | Anthropic | `anthropic` | Messages API，适合 Claude Code。 |
| `GET /v1/models` | OpenAI 风格 | 聚合 | 返回所有启用渠道中已添加的客户端模型。 |

请求必须提供有效 PhonyG 用户 Key。OpenAI 客户端通常使用 `Authorization: Bearer ...`；Anthropic 客户端可使用 `x-api-key`。PhonyG 验证用户 Key 后，会移除客户端鉴权并按渠道协议写入上游鉴权。

## 管理路径

- `GET /api/health`：无需 JWT，返回 `{"status":"ok"}`。
- `/api/setup/*`：首次管理员初始化相关接口。
- `/api/auth/login`：管理员登录。
- 其余 `/api/*`：要求 `Authorization: Bearer ADMIN_JWT`。

## 模型选路

1. 从请求体读取顶层 `model`。
2. 按请求路径确定 OpenAI/Anthropic 协议。
3. 查找协议匹配、渠道启用、未临时禁用、客户端模型匹配的候选渠道。
4. 数字越大的 `priority` 越优先；同优先级随机选择。
5. 映射开启模型改写时，只替换顶层 `model`，其余 Body 字节保持不变。

## Header 处理

- `透传` 保留非鉴权、非 hop、非传输业务 Header；
- 渠道额外 Header 随后应用；
- `预设` / `自定义` 按规则移除和覆盖业务 Header；
- 上游鉴权、Host、Content-Length、Accept-Encoding 和 hop-by-hop Header 由网关控制。

## 错误结构

网关本地错误使用：

```json
{
  "error": {
    "message": "human readable message",
    "type": "gateway_error",
    "code": "model_not_found"
  }
}
```

每个请求返回 `X-Request-Id`，可用于和请求日志对照。
