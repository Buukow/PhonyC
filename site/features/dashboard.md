---
layout: default
title: 概览与统计
parent: 功能指南
nav_order: 1
permalink: /features/dashboard/
---

# 概览与统计

概览页用于快速判断网关流量、错误和 Token 变化，不替代详细请求日志。

<figure class="screenshot">
  <img src="{{ '/assets/images/dashboard.png' | relative_url }}" alt="PhonyG 概览页">
  <figcaption>请求量、成功率、Token 趋势、热门模型和近期请求。</figcaption>
</figure>

## 指标区域

- 请求总数与成功/错误状态；
- Token 总量与输入、输出拆分；
- 按时间绘制的请求量、Token 趋势；
- 热门客户端模型；
- 近期请求与异常摘要。

捕获 Key 产生的 capture-only 请求会进入请求元数据，但 Token 为 0、没有普通用户 Key 和渠道，因此不会制造虚假的上游 Token 用量。

## 如何使用

1. 先看错误率是否突然上升；
2. 对照 Token 趋势判断是否是流量增长；
3. 查看热门模型，确认客户端模型命名符合预期；
4. 进入 [请求日志]({{ '/features/logs/' | relative_url }}) 按状态、Key、渠道或路径筛选。
