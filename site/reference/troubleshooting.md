---
layout: default
title: 故障排查
parent: 配置参考
nav_order: 4
permalink: /reference/troubleshooting/
---

# 故障排查

## 管理台打不开

- 检查 `PHONYG_ADDR` 是否监听 `0.0.0.0` 而不是容器内 loopback；
- 检查宿主机端口映射和防火墙；
- 先请求 `GET /api/health` 区分服务故障和前端缓存问题；
- 浏览器硬刷新，确认嵌入的前端资源与二进制版本一致。

## 登录反复跳回登录页

- JWT 已超过 `PHONYG_JWT_TTL_HOURS`；
- `PHONYG_JWT_SECRET` 或数据目录里的 `jwt_secret` 在重启时变化；
- 浏览器 localStorage 中保存了旧环境的 `phonyg_token`。

## `/v1/models` 没有模型

模型必须被明确添加到渠道的模型表并启用。只从上游拉取列表但没有点击添加，不会进入聚合列表，也不会参与测活和选路。

## 返回 `model_not_found`

- 顶层 `model` 不等于任何启用映射的客户端模型；
- 路径协议与渠道协议不匹配；
- 匹配渠道被手动停用或测活临时禁用；
- 自动重试已排除所有失败候选。

## 渠道被临时禁用

正式流量或自动测活命中配置状态码（默认 `401,403,404,503`）时会临时禁用。修复上游后可等待下一轮自动测活恢复、对该渠道手动测活，或在渠道列表手动启用以清除状态。

## 预设保存失败

常见原因包括：受保护 Header、无效 JSON、未知模板、生成器引用不存在、依赖循环、无效子字段路径、递增模式用于非数字生成器。

## 捕获 Key 返回 403

捕获已完成或尚未重新布防。进入 **请求捕获** 页面点击重新布防，再发下一次捕获请求。不要把捕获 Key 当作普通代理 Key。

## 文档站资源 404

GitHub 项目页必须使用 `/PhonyC` baseurl。文档图片和内部链接使用 Jekyll `relative_url`，本地预览也应带 `--baseurl /PhonyC` 并访问 `/PhonyC/`。
