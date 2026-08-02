---
layout: default
title: Docker 部署
parent: 快速开始
nav_order: 2
permalink: /docker/
---

# Docker 部署

Docker 镜像包含已经构建好的 React 管理台和静态 Go 二进制，以非 root 用户运行，默认监听容器内 `8080`，数据目录为 `/data`。

## 使用预构建镜像

```bash
docker pull ghcr.io/buukow/phonyg:1.9

docker run -d \
  --name phonyg \
  --restart unless-stopped \
  -p 8080:8080 \
  -v phonyg-data:/data \
  ghcr.io/buukow/phonyg:1.9
```

访问 `http://HOST:8080/` 初始化管理员。

{: .tip }
生产环境建议固定 `1.9` 或具体版本标签。`latest` 会跟随默认分支的新镜像，适合持续跟进项目但不利于精确回滚。

## Docker Compose

下面的完整示例显式配置运行参数和健康检查：

```yaml
services:
  phonyg:
    image: ghcr.io/buukow/phonyg:1.9
    container_name: phonyg
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      PHONYG_ADDR: "0.0.0.0:8080"
      PHONYG_DATA_DIR: "/data"
      PHONYG_JWT_TTL_HOURS: "24"
      PHONYG_MAX_BODY_BYTES: "67108864"
      # PHONYG_JWT_SECRET: "replace-with-a-long-random-secret"
    volumes:
      - phonyg-data:/data
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8080/api/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  phonyg-data:
```

当前运行镜像基于 `debian:bookworm-slim`，只包含 CA 证书、时区数据和 PhonyG；若镜像中没有 `wget`，可从宿主机或反向代理执行健康检查，或者在自定义镜像中增加探针工具。

## 最简 Docker Compose

只保留启动服务所需的镜像、重启策略、端口和持久化数据卷：

```yaml
services:
  phonyg:
    image: ghcr.io/buukow/phonyg:1.9
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - phonyg-data:/data

volumes:
  phonyg-data:
```

保存为 `compose.yml` 后运行：

```bash
docker compose up -d
```

## 环境变量

| 变量 | 默认值 | Docker 推荐值 | 说明 |
|---|---:|---:|---|
| `PHONYG_ADDR` | `:8080` | `0.0.0.0:8080` | HTTP 监听地址。容器内必须监听非 loopback 才能通过端口映射访问。 |
| `PHONYG_DATA_DIR` | `./data` | `/data` | SQLite 数据库和自动生成的 JWT secret 所在目录。 |
| `PHONYG_JWT_SECRET` | 空，自动生成 | 长随机字符串或留空持久化 | 非空时直接作为 JWT HMAC 密钥；留空时写入数据目录的 `jwt_secret`。 |
| `PHONYG_MAX_BODY_BYTES` | `67108864`（64 MiB） | 按客户端请求大小调整 | 接收代理请求体的字节上限；非法值或非正数回退到 64 MiB。 |
| `PHONYG_JWT_TTL_HOURS` | `24` | `24` | 管理员 JWT 有效小时数；非正数回退到 24。 |

更完整的类型与行为参见 [环境变量参考]({{ '/reference/environment/' | relative_url }})。

## 自行构建镜像

```bash
docker build -t phonyg:local .
docker run -d --name phonyg-local \
  -p 8080:8080 \
  -v phonyg-local-data:/data \
  phonyg:local
```

Dockerfile 使用三阶段构建：Node.js 构建前端、Go 构建静态 `linux/amd64` 二进制、Debian slim 作为运行时。
