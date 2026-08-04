---
layout: default
title: 首页
nav_order: 1
permalink: /
---

<div class="hero">
  <h1>PhonyG</h1>
  <p>轻量 AI API 中转网关：在不破坏请求体的前提下完成协议路由、模型映射、客户端 Header 伪装、自动测活、失败重试与请求观测。</p>
  <a class="btn btn-primary" href="{{ '/getting-started/' | relative_url }}">快速开始</a>
  <a class="btn" href="{{ '/docker/' | relative_url }}">Docker 部署</a>
</div>

## 为什么使用 PhonyG

PhonyG 以 Go 提供单进程网关，React 管理台嵌入二进制，SQLite 保存配置和请求元数据。它适合需要统一接入多个 OpenAI/Anthropic 兼容上游，并希望模拟 Codex、Claude Code 等真实客户端请求特征的单机部署。

<div class="feature-grid">
  <a class="feature-card" href="{{ '/features/channels/' | relative_url }}"><strong>协议与渠道路由</strong><span>OpenAI / Anthropic 路径、优先级、模型映射和临时禁用。</span></a>
  <a class="feature-card" href="{{ '/features/presets/' | relative_url }}"><strong>客户端指纹预设</strong><span>结构化 Header、动态会话生成器、强制覆盖与缺失补全。</span></a>
  <a class="feature-card" href="{{ '/features/healthcheck/' | relative_url }}"><strong>自动测活增强</strong><span>随机提示词、stream-first、非流式回退与自动恢复。</span></a>
  <a class="feature-card" href="{{ '/features/capture/' | relative_url }}"><strong>请求捕获</strong><span>捕获真实客户端业务 Header，一键转换为可复用预设。</span></a>
</div>

## 核心能力

- **Header 重组 + Body 玻璃穿透**：只在需要时对顶层 `model` 做字节级改写，其余请求体保持原样。
- **两类上游协议**：OpenAI 路径和 Anthropic `/v1/messages` 分别使用对应鉴权方式。
- **三种 Key 伪装模式**：透传、预设、自定义。
- **运行观测**：概览、模型热度、状态码、延迟、输入/输出/总 Token、错误摘要和筛选日志。
- **单机友好**：默认 SQLite，无外部数据库依赖；管理台由 JWT 保护。

## 选择安装方式

| 场景 | 推荐页面 |
|---|---|
| 第一次体验 | [快速开始]({{ '/getting-started/' | relative_url }}) |
| 修改源码、调试前后端 | [本地构建]({{ '/local-build/' | relative_url }}) |
| 稳定运行、便于升级 | [Docker 部署]({{ '/docker/' | relative_url }}) |

{: .note }
公开仓库名仍为 `PhonyC`，产品和镜像名称已经统一为 **PhonyG**。
