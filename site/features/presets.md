---
layout: default
title: 客户端预设 ⭐
parent: 功能指南
nav_order: 4
permalink: /features/presets/
---

# 客户端预设 ⭐

客户端预设把真实客户端的静态指纹、动态会话字段和 Header 合并行为组织成可复用规则，并绑定到用户 Key。

<figure class="screenshot">
  <img src="{{ '/assets/images/presets.png' | relative_url }}" alt="PhonyG 客户端预设列表">
  <figcaption>四个内置 Codex / Claude Code 预设和自定义预设。</figcaption>
</figure>

## 内置预设

| 预设 | 用途 |
|---|---|
| `codex-tui` | Codex CLI 核心静态请求头。 |
| `codex-enhanced` | Codex 静态指纹 + 相关联的 Session、Thread、Turn 和窗口元数据。 |
| `claude-cli` | Claude Code 核心 Anthropic/Stainless SDK 请求头。 |
| `claude-enhanced` | Claude Code 静态指纹 + 动态 `X-Claude-Code-Session-Id`。 |

内置预设不能原地修改。点击编辑可查看完整规则，保存时需要另取名称，创建独立自定义预设。

## 强制覆盖与缺失补全

每个顶层 Header 和嵌套字段都有一个两态按钮开关：

- **右侧 / 默认：强制覆盖**（`fill_missing: false`）——客户端有同名值也替换，没有则添加；
- **左侧：缺失补全**（`fill_missing: true`）——客户端已有值时保留，只有缺失时才添加。

四个内置预设全部默认强制覆盖，以稳定模拟对应真实客户端，而不是被用户请求中的同名 Header 改变指纹。

<figure class="screenshot">
  <img src="{{ '/assets/images/preset-editor.png' | relative_url }}" alt="PhonyG 预设树编辑器和覆盖开关">
  <figcaption>父字段定义默认模式；子字段可继承或保存显式覆盖设置。</figcaption>
</figure>

### 父子字段

对象型 Header（例如 JSON 格式的 `X-Codex-Turn-Metadata`）可以逐层配置：

- 子字段没有显式配置时显示“继承：强制覆盖”或“继承：缺失补全”；
- 拨动子字段开关会在 `children_fill_missing` 中创建显式规则；
- “恢复继承”删除显式规则；
- 修改父字段不会删除子字段的自定义值；
- 父行出现“含自定义子项”表示至少一个后代与继承模式不同。

数组按数字索引处理缺失补全。客户端对象 Header 不是有效 JSON 时，请求会返回明确错误，不会静默拼出矛盾结果。

## 结构化规则

{% raw %}
```json
{
  "schema_version": 1,
  "headers": {
    "User-Agent": {
      "value": "codex_exec/{{version}}",
      "fill_missing": false
    },
    "X-Codex-Turn-Metadata": {
      "value": {
        "session_id": "{{generator:session}}",
        "turn_id": "{{generator:turn}}"
      },
      "fill_missing": false,
      "children_fill_missing": {
        "session_id": true
      }
    }
  },
  "remove_headers": [],
  "generators": {}
}
```
{% endraw %}

可视化编辑与 JSON 编辑指向同一份规范文档。无效 JSON、未知模板、循环依赖、无效生成器或受保护 Header 会拒绝保存。

## 模板与引用

{% raw %}
- `{{version}}`：预设版本标签；
- `{{client_header:Header-Name}}`：读取未经修改的客户端 Header；
- `{{resolved_header:Header-Name}}`：读取前序解析后的 Header；
- `{{generator:name}}`：读取命名生成器；
- `{{time:unix_ms}}`、`{{time:year}}` 等：读取同一请求时间快照。
{% endraw %}

依赖图会检测直接和间接循环。受保护 Header 不能被引用来间接写入规则结果。

## 动态生成器

生成器支持 UUID v4/v7 和固定长度随机字符，刷新模式包括：

- **每请求随机**：同一请求中复用，下一请求重新生成；
- **运行期固定**：进程启动生成一次；
- **定时刷新**：达到滚动时间间隔后刷新；
- **递增序列**：数字初值按步长递增，支持循环、重新随机、扩位或报错。

内置增强预设利用共享生成器保持多个 Header 中的 Session ID 一致，同时每个请求生成新的 Turn ID 和时间戳。

## 受保护 Header

预设不能写入、删除或引用：`Authorization`、`X-Api-Key`、`Host`、`Content-Length`、`Accept-Encoding` 以及 `Connection`、`Transfer-Encoding` 等 hop-by-hop Header。上游鉴权和传输字段始终由网关控制。

## 请求捕获生成预设

捕获结果保存为 schema v1：每个业务 Header 成为顶层规则，默认 **强制覆盖**，不自动创建生成器。之后可以在可视化编辑器中把会话字段替换成动态生成器。
