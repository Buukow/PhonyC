# PhonyC

轻量 AI API 中转网关（Go + React 管理台）。

## 特性

- Header 重组 + Body 玻璃穿透
- 可选字节级 `model` 改写
- openai / anthropic 协议渠道
- 用户 Key 三模式伪装（passthrough / preset / custom）
- SQLite 单机、JWT 管理台、请求元数据与仪表盘

## 快速开始

```bash
make build
PHONYC_ADDR=:8080 PHONYC_DATA_DIR=./data ./bin/phonyc
```

打开 http://127.0.0.1:8080 完成首次管理员初始化。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `PHONYC_ADDR` | `:8080` | 监听地址 |
| `PHONYC_DATA_DIR` | `./data` | 数据目录 |
| `PHONYC_JWT_SECRET` | 自动生成 | JWT 密钥 |
| `PHONYC_MAX_BODY_BYTES` | 64MB | 请求体上限 |

### 开发

```bash
# 后端
go run ./cmd/phonyc

# 前端
cd web && npm run dev
```

## 代理路径

- `POST /v1/chat/completions` ` /v1/completions` ` /v1/responses` → openai 渠道
- `POST /v1/messages` → anthropic 渠道
- `GET /v1/models` → 聚合模型列表

管理 API 前缀：`/api/*`（需管理员 JWT，除 setup/login/health）。
