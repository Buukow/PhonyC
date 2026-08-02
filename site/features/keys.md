---
layout: default
title: 用户 Key 与伪装模式
parent: 功能指南
nav_order: 3
permalink: /features/keys/
---

# 用户 Key 与伪装模式

用户 Key 是客户端访问 PhonyG 代理路径的凭证，同时绑定 Header 伪装策略。

<figure class="screenshot">
  <img src="{{ '/assets/images/keys.png' | relative_url }}" alt="PhonyG 用户 Key 页面">
  <figcaption>创建、复制、启停用户 Key，并选择 passthrough、preset 或 custom。</figcaption>
</figure>

## 三种模式

### passthrough

保留客户端业务 Header，过滤客户端鉴权、Host、Content-Length、Accept-Encoding 和 hop-by-hop Header，再写入渠道鉴权和额外 Header。适合先验证协议和路由。

### preset

绑定结构化客户端预设。适合模拟 Codex、Claude Code 或捕获到的真实客户端指纹。规则可以删除、补全或强制覆盖业务 Header，并生成动态会话字段。

### custom

用简单 JSON Header Map 覆盖业务 Header，适合不需要生成器和嵌套字段规则的少量自定义。

## 生命周期

- 新建时 Key 留空会自动生成 `sk-...`；
- 编辑时 Key 留空保持原值；
- 停用后所有代理请求被拒绝；
- 删除 Key 不会删除历史请求元数据中的统计记录；
- 备注适合记录使用者、环境或用途。

{: .warning }
用户 Key 与“请求捕获”页面的系统固定捕获 Key 不同。捕获 Key 只用于捕获下一次请求，不能作为普通上游代理凭证。
