---
layout: default
title: 本地构建
nav_order: 3
permalink: /local-build/
---

# 本地构建

本地构建适合开发、调试、修改管理台或编译自己的 PhonyG 二进制。

## 构建要求

- Go 1.26 或与 `go.mod` 兼容的版本；
- Node.js 20 与 npm；
- Linux、macOS，或可以运行 Go/Node 的开发环境。

## 一条命令构建

```bash
make build
```

这个命令会：

1. 在 `web/` 安装前端依赖；
2. 使用 Vite 构建 React 管理台；
3. 将产物写入 `internal/webembed/dist/`；
4. 通过 Go `embed` 打包进 `bin/phonyg`。

启动：

```bash
PHONYG_ADDR=0.0.0.0:23342 \
PHONYG_DATA_DIR=./data \
./bin/phonyg
```

## 分步开发

后端：

```bash
go run ./cmd/phonyg
```

前端：

```bash
cd web
npm install
npm run dev
```

前端开发服务器只用于 UI 开发；正式二进制使用的是构建后嵌入的静态资源。

## 测试

```bash
go test ./... -count=1
cd web
npm test -- --run
npm run build
```

## 数据目录

`PHONYG_DATA_DIR` 中包含：

- `phonyg.db`：SQLite 数据库；
- `phonyg.db-wal` / `phonyg.db-shm`：SQLite WAL 文件（运行中可能存在）；
- `jwt_secret`：未设置 `PHONYG_JWT_SECRET` 时自动生成的管理 JWT 密钥。

{: .warning }
升级或迁移前，应停止写入或使用 SQLite 在线备份，不能只复制正在变化的单个 `phonyg.db` 文件而忽略 WAL。

下一页：[Docker 部署]({{ '/docker/' | relative_url }})。
