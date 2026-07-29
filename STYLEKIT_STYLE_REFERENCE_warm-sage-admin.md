STYLEKIT_STYLE_REFERENCE
style_name: 暖米青绿后台
style_slug: warm-sage-admin
style_source: /styles/warm-sage-admin

# Creative Brief

## 什么时候用
当你需要设计或生成 **SaaS / 数据分析 / 运营管理后台**，并希望整体呈现「温润办公」气质时使用。它保留暖米白画布与青绿主色的核心识别度，允许布局与组件实现按页面灵活调整。

## 怎么用
- 还在探索后台信息架构或视觉方向时，把本文复制到 AI 工具里。
- 补充页面类型（概览 / 分析 / 用户 / 报告 / 设置）、目标用户与业务约束。
- 先让 AI 给 2-3 个方向（例如：更卡片化、更表格驱动、更窄栏设置流），确定方向后再用硬性约束落地。
- 实现阶段优先复用本文的色板、圆角、阴影与字阶；图表、表格字段可按数据增减。

保持整体风格气质即可，允许实现细节灵活调整，但不要偏离核心视觉语言。

## Style Signals

- 温润
- 克制
- 暖米白
- 青绿主色
- 轻卡片
- 专业办公
- 中文后台
- 低对比阴影

## Prefer

- 页面与侧栏底色使用暖米白 `bg-[#faf8f5]`；卡片与浮层用纯白 `bg-white`
- 主色使用柔和青绿 `#4a9d9a`：主按钮、侧栏选中、链接、焦点环、进度条、默认图表柱
- 点缀色：金沙 `#e8b86d`（次级强调 / 图表 hover / 部分头像）、陶土 `#c17767`（负向 / 警告 / 通知红点）、中性青绿灰 `#6b8e8e`（完成 / 中性指标）
- 正文 `text-gray-800` / `text-gray-700`，说明 `text-gray-400` / `text-gray-500`，极弱辅助 `text-gray-300`
- 边框用细灰 `border-gray-200` / `border-gray-100` / `border-gray-50`，顶栏与侧栏可用半透明 `border-gray-200/50~60`
- 主色透明用法：`bg-[#4a9d9a]/10`、`ring-[#4a9d9a]/30`、`shadow-[#4a9d9a]/20~25`；状态 pill 用「色 + 10% 底 + 同色字」
- 卡片 `rounded-2xl` + `shadow-xl shadow-black/[0.04]`；可交互卡 `hover:shadow-2xl hover:-translate-y-1 duration-300`
- 按钮、输入、图标容器、搜索、单字头像块统一 `rounded-xl`；小 pill / tooltip `rounded-lg`；进度条与红点 `rounded-full`
- 主 CTA：`bg-[#4a9d9a] text-white` + `shadow-lg shadow-[#4a9d9a]/25`，hover 微抬 `-translate-y-0.5`
- 输入与筛选：`bg-[#faf8f5] border-gray-200 rounded-xl`，focus `ring-2 ring-[#4a9d9a]/30 border-[#4a9d9a]`
- 布局：固定侧栏约 `w-60` 可折叠、主区同步 `ml-60`、顶栏 `sticky` + `bg-[#faf8f5]/80 backdrop-blur-md`、内容 `p-6 md:p-8`
- 字阶：页标题 `text-xl font-semibold`；卡标题 `text-lg font-semibold`；卡副标题 `text-xs text-gray-400`；表头 `text-xs uppercase tracking-wider text-gray-400`
- 图标用 lucide 一类细线图标，尺寸约 `w-4 h-4` / `w-5 h-5`，颜色跟灰阶或主色
- 表格行 hover 回暖米白 `hover:bg-[#faf8f5]`；分段控件（如 7d/30d/90d）用浅底 pill 容器 + 选中白底轻阴影
- 动效安静：`transition-all duration-200~300`，侧栏开合 300ms；通知、下拉可点外部关闭
- 文案与样例优先中文业务后台语境（概览、数据分析、用户管理、报告、设置）

## Avoid

- 禁止冷蓝科技风、霓虹、赛博紫、厚重玻璃拟态作为主视觉
- 禁止大面积高饱和渐变英雄区或炫光
- 禁止纯黑 `#000000` 大块铺底；正文用 gray-800 而非死黑
- 禁止直角硬边、粗黑描边、重黑投影（`shadow-black/40+` 一类）
- 禁止把主色换成默认 Tailwind 亮蓝 / indigo 作为品牌色
- 禁止拥挤无留白的密集企业灰界面；保持卡片分区与暖色呼吸感
- 禁止夸张 3D、拟物装饰盖过可读性
- 禁止用完美对称的营销 Landing 语法硬套后台（除非单独说明）

## Output Guidance

- 先保证「暖米白 + 青绿主色 + 白卡片轻阴影」的整体识别度，再优化表格字段、图表数据与交互细节。
- 探索阶段可调整栅格与模块组合；落地阶段色板、圆角层级、阴影与 focus 环尽量固定。
- 优先可读性与可维护的 Tailwind / 组件结构，避免为风格牺牲对比度与操作可达性。
- 需要变体时，可在不改核心色板的前提下调整：更偏表格、更偏指标卡、或更窄的设置表单流。
- 输出实现时注明关键 token（canvas / primary / accent / warn / surface）便于后续 Stylekit 复用。

## Token Snapshot（可选硬性参考）

| Token | Value | Role |
|-------|-------|------|
| canvas | `#faf8f5` | 页/侧栏/表单浅底、行 hover |
| surface | `#ffffff` | 卡片、下拉、浮层 |
| primary | `#4a9d9a` | 品牌、CTA、选中、进度、焦点 |
| accent | `#e8b86d` | 次级强调、图表 hover |
| warn | `#c17767` | 负向、警告、红点 |
| muted | `#6b8e8e` | 完成/中性 |
| ink | `gray-800` | 主文字 |
| quiet | `gray-400` | 说明与表头 |

## One-liner

暖米白画布 `#faf8f5` + 青绿主色 `#4a9d9a` + 金沙/陶土点缀；白底 `rounded-2xl` 轻阴影卡片与 `rounded-xl` 控件；克制 hover 微抬的中文 SaaS 后台，不要冷蓝、不要霓虹。
