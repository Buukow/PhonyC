---
layout: default
title: 快速开始
nav_order: 2
permalink: /getting-started/
---

# 快速开始

本页用最少步骤启动 PhonyG、完成管理员初始化、创建渠道和用户 Key，然后发出第一条请求。

## 1. 启动服务

如果已经安装 Go 与 Node.js：

```bash
git clone https://github.com/Buukow/PhonyC.git
cd PhonyC
make build
PHONYG_ADDR=0.0.0.0:8080 PHONYG_DATA_DIR=./data ./bin/phonyg
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/health
# {"status":"ok"}
```

也可以直接使用 [Docker 部署]({{ '/docker/' | relative_url }})，无需在主机安装 Go 和 Node.js。

## 2. 初始化管理员

浏览器打开 `http://127.0.0.1:8080/`。首次运行会进入初始化页：

1. 设置管理员用户名。
2. 设置至少 6 位密码。
3. 登录管理台。

管理员会话使用 JWT。默认有效期 24 小时，可通过 `PHONYG_JWT_TTL_HOURS` 调整。

## 3. 创建上游渠道

进入 **渠道 → 新建渠道**：

- 选择 `openai` 或 `anthropic` 协议；
- 填写上游 Base URL 和 API Key；
- 设置优先级和超时；
- 从上游 `/v1/models` 拉取模型，或手动添加客户端模型与上游模型映射；
- 保存后按需手动测活。

只有已经添加并启用的模型映射会出现在网关 `/v1/models`，也只有这些模型会参与选路与自动测活。

## 4. 创建用户 Key

进入 **用户 Key → 新建 Key**：

- Key 留空时由系统自动生成；
- 初次接入可选择 `passthrough`；
- 模拟 Codex/Claude Code 时选择 `preset` 并绑定预设；
- 简单覆盖少量 Header 时选择 `custom`。

## 5. 发出请求

OpenAI Chat Completions：

```bash
curl http://127.0.0.1:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-YOUR-PHONYG-KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "YOUR-CLIENT-MODEL",
    "messages": [{"role":"user","content":"hi"}],
    "stream": false
  }'
```

OpenAI Responses：

```bash
curl http://127.0.0.1:8080/v1/responses \
  -H 'Authorization: Bearer sk-YOUR-PHONYG-KEY' \
  -H 'Content-Type: application/json' \
  -d '{"model":"YOUR-CLIENT-MODEL","input":"hi"}'
```

Anthropic Messages：

```bash
curl http://127.0.0.1:8080/v1/messages \
  -H 'x-api-key: sk-YOUR-PHONYG-KEY' \
  -H 'anthropic-version: 2023-06-01' \
  -H 'content-type: application/json' \
  -d '{
    "model":"YOUR-CLIENT-MODEL",
    "max_tokens":128,
    "messages":[{"role":"user","content":"hi"}]
  }'
```

{: .tip }
客户端请求体里的模型名必须与渠道模型表中的“客户端模型”一致。是否改写为另一上游模型，由该映射的 `rewrite_model` 配置决定。

## 下一步

- 需要修改源码：阅读 [本地构建]({{ '/local-build/' | relative_url }})。
- 准备长期部署：阅读 [Docker 部署]({{ '/docker/' | relative_url }})。
- 需要模拟客户端：阅读 [客户端预设]({{ '/features/presets/' | relative_url }})。
- 想捕获真实请求头：阅读 [请求捕获]({{ '/features/capture/' | relative_url }})。
