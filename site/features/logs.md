---
layout: default
title: 请求日志
parent: 功能指南
nav_order: 6
permalink: /features/logs/
---

# 请求日志

请求日志保存元数据而不是完整请求体，适合定位路由、状态、耗时和 Token 问题。

<figure class="screenshot">
  <img src="{{ '/assets/images/logs.png' | relative_url }}" alt="PhonyG 请求日志">
  <figcaption>按关键词、路径、Key、渠道和状态码筛选请求。</figcaption>
</figure>

## 可筛选字段

- 关键词；
- 请求路径；
- 用户 Key；
- 渠道；
- HTTP 状态码。

## 日志列

| 列 | 含义 |
|---|---|
| Key | 普通用户 Key 的数据库 ID；捕获请求为空。 |
| 路径 | 客户端调用的网关 API 路径。 |
| 模型 | 请求体中的客户端模型。 |
| 渠道 | 最终尝试的渠道 ID；本地拒绝和捕获请求为空。 |
| 状态 | 返回给客户端或上游响应的状态码。 |
| 耗时 | 请求从进入网关到完成的总耗时。 |
| 输入/输出/合计 | 从成功响应中嗅探到的 Token；错误和捕获请求通常为 0。 |
| 摘要 | 本地错误、上游错误或重试原因；成功请求为空。 |

自动重试的失败尝试也可能写入元数据，摘要包含 `retrying`；最终成功/失败会有后续记录。日志保留天数在设置页配置。

## 捕获请求

请求捕获成功时会出现 capture-only 记录：状态 200，无普通用户 Key、无渠道、Token 全部为 0、错误摘要为空。它证明请求已由 PhonyG 处理，但不代表访问过上游。
