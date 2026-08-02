---
layout: default
title: 快速开始
nav_order: 2
has_children: true
permalink: /getting-started/
---

# 快速开始

根据你的使用场景，从下面三个页面开始：

## [本地构建]({{ '/local-build/' | relative_url }})

适合开发、调试、修改管理台，或从源码编译自己的 PhonyG 二进制。

## [Docker 部署]({{ '/docker/' | relative_url }})

使用预构建镜像、Docker Compose 或本地镜像部署 PhonyG，并配置容器环境变量和持久化数据卷。

## [如何使用]({{ '/how-to-use/' | relative_url }})

服务启动后，从初始化管理员开始，依次创建上游渠道、用户 Key，并发出第一条 OpenAI 或 Anthropic 请求。
