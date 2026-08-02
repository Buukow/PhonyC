---
layout: default
title: 请求捕获 ⭐
parent: 功能指南
nav_order: 5
permalink: /features/capture/
---

# 请求捕获 ⭐

请求捕获用于记录 Codex、Claude Code 或其他客户端发给 PhonyG 的过滤后完整业务 Header，再一键保存为客户端预设。捕获 Key 的请求 **只捕获不转发**（capture-only），不会访问任何上游。

## 四步完成捕获

<ol class="step-list">
  <li><strong>开启捕获。</strong>进入侧栏“请求捕获”，点击开启。系统生成或复用固定捕获 API Key，并进入“等待首次请求”布防状态。</li>
  <li><strong>让 AI 客户端调用 PhonyG。</strong>把客户端 Base URL 指向 PhonyG，Authorization 或 API Key 改为页面显示的固定捕获 Key，然后发起一次对话。</li>
  <li><strong>确认客户端返回。</strong>PhonyG 按 OpenAI/Anthropic 路径返回协议适配后的成功内容，客户端应显示 <code>captured</code>。</li>
  <li><strong>查看并保存。</strong>管理台显示捕获到的业务 Header；填写预设名后一键保存。同名自定义预设可覆盖，内置预设不会被覆盖。</li>
</ol>

<figure class="screenshot">
  <img src="{{ '/assets/images/capture.png' | relative_url }}" alt="PhonyG 请求捕获页面">
  <figcaption>捕获开关、布防状态、系统固定 API Key 和保存预设入口。</figcaption>
</figure>

## 请求流转

```text
AI 客户端
  → PhonyG 固定捕获 Key
  → 过滤鉴权 / Cookie / Host / Content-Length / 压缩 / hop-by-hop Header
  → 保存下一次请求的业务 Header
  → 写入 capture-only 请求元数据
  → 返回 captured（不访问上游）
```

## 成功返回

<figure class="screenshot">
  <img src="{{ '/assets/images/capture-client-response.png' | relative_url }}" alt="AI 客户端显示 captured 成功响应">
  <figcaption>AI 客户端收到 <code>captured</code>，证明请求已被 PhonyG 捕获。此图位由用户提供的客户端截图替换。</figcaption>
</figure>

成功时，AI 客户端正常结束本次调用，管理台由“等待首次请求”切换为“已捕获/未布防”。这不表示上游模型生成了回复；它只表示 PhonyG 已完成捕获。

## 捕获结果

<figure class="screenshot">
  <img src="{{ '/assets/images/capture-headers.png' | relative_url }}" alt="PhonyG 捕获到的客户端业务 Header">
  <figcaption>捕获结果保留客户端标识、SDK 指纹和 Session-Id 等会话级业务 Header。</figcaption>
</figure>

常见保留字段包括：

- Codex：`Session-Id`、`Thread-Id`、`X-Client-Request-Id`、`X-Codex-Turn-Metadata`、`X-Codex-Window-Id`；
- Claude Code：`X-Claude-Code-Session-Id`、Anthropic/Stainless SDK 指纹 Header；
- 普通业务 Header：`User-Agent`、`Accept`、客户端版本与功能标记。

### 哪些 Header 会过滤

捕获不会保存 `Authorization`、`X-Api-Key`、`Cookie`、`Host`、`Content-Length`、`Accept-Encoding`、`Connection`、`Keep-Alive`、`Transfer-Encoding`、`TE`、`Trailer`、`Upgrade`、代理鉴权和其他 hop-by-hop/传输字段。

{: .warning }
这里的“完整”指过滤范围内的完整业务 Header 名和值。当前捕获存储是 `map[string]string`：同名 Header 出现多个值时只保留第一个值（首值），不会保存重复 Header 的全部多值实例。

## 布防与 403

- 开启捕获会立即布防并清除旧捕获；
- 布防状态只记录下一次使用固定 Key 的请求；
- 捕获完成后自动取消布防；
- 要捕获下一次请求，点击“重新布防”；
- 捕获关闭或未布防时继续使用该固定 Key，会返回 **403**；
- 普通用户 Key 不受捕获页面布防状态影响。

## 保存为客户端预设

每个捕获 Header 转换为 schema v1 顶层规则，默认使用 **强制覆盖**。系统不会猜测哪些值应该动态变化，也不会自动创建 Session 生成器。建议另存后进入 [客户端预设]({{ '/features/presets/' | relative_url }})：

1. 将静态客户端指纹保留为强制覆盖；
2. 把 Session/Thread/Turn 等值替换为 UUID 或时间生成器；
3. 对确实应该尊重客户端值的字段改为缺失补全；
4. 使用预览确认最终 Header。

## 请求日志语义

捕获请求也会出现在 PhonyG 请求日志中，但它不是普通代理调用：

| 字段 | 值 |
|---|---|
| 状态 | `200` |
| 客户端模型 | 从请求体读取；没有时使用捕获占位模型 |
| 用户 Key | 空（固定捕获 Key 不创建普通用户 Key 统计） |
| 渠道 | 空 |
| Token | 输入、输出、合计均为 0（零 Token） |
| 错误摘要 | 空字符串 |
| 伪装模式 | `passthrough` 元数据 |

日志写入是 best-effort；即使本地日志写入异常，也不会把捕获请求转发到上游。
