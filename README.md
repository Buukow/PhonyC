# PhonyG

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
PHONYG_ADDR=:8080 PHONYG_DATA_DIR=./data ./bin/phonyg
```

打开 http://127.0.0.1:8080 完成首次管理员初始化。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `PHONYG_ADDR` | `:8080` | 监听地址 |
| `PHONYG_DATA_DIR` | `./data` | 数据目录 |
| `PHONYG_JWT_SECRET` | 自动生成 | JWT 密钥 |
| `PHONYG_MAX_BODY_BYTES` | 64MB | 请求体上限 |

### 开发

```bash
# 后端
go run ./cmd/phonyg

# 前端
cd web && npm run dev
```

## 代理路径

- `POST /v1/chat/completions` ` /v1/completions` ` /v1/responses` → openai 渠道
- `POST /v1/messages` → anthropic 渠道
- `GET /v1/models` → 聚合模型列表

管理 API 前缀：`/api/*`（需管理员 JWT，除 setup/login/health）。


## 自动测活

设置页可开启自动测活：

- 间隔分钟 + 随机偏移（每轮等待 = 间隔 + `0..偏移` 分钟）
- 提问词默认 `hi`，可自定义
- 模型：每个渠道**模型表第一个启用映射**
- 仅测手动启用的渠道（含临时禁用，用于恢复）
- 命中配置状态码（默认 401/403/404/503）→ 临时禁用；下次 2xx → 恢复
- 临时禁用不参与代理选路；渠道列表只显示一个明确状态：启用、停用或临时禁用
- 手动测活允许测试所有渠道：停用/启用渠道只记录结果，临时禁用渠道仅在成功时恢复
- 用户点击临时禁用渠道的「启用」可强制清除临时禁用状态

## 请求捕获

侧栏「请求捕获」：

- 开启后使用系统固定 API Key
- 布防中：该 Key **只捕获不转发**，返回 `{"captured":true}`
- 记录第一次请求的客户端业务头（去掉鉴权与 hop/传输头）
- 可一键保存/覆盖为客户端预设；重新布防后捕获下一次

## 优先级

渠道 `priority`：**0 为默认最低**，数字越大越优先；同优先级随机；禁止负数。

## Deploy (port 23342)

```bash
./scripts/deploy.sh
# or
make deploy
systemctl status phonyg
```

Access URLs after deploy:
- http://127.0.0.1:23342/
- http://172.16.0.106:23342/
- http://202.189.7.62:23342/

Service is managed by systemd unit `phonyg.service` with `Restart=always`.


## Docker

```bash
docker pull ghcr.io/buukow/phonyg:1.2
docker run -d --name phonyg \
  -p 8080:8080 \
  -v phonyg-data:/data \
  ghcr.io/buukow/phonyg:1.2
```

镜像由 GitHub Actions 构建（`linux/amd64`），版本标签：`1.2`。
