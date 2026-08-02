---
layout: default
title: 环境变量
parent: 配置参考
nav_order: 1
permalink: /reference/environment/
---

# 环境变量

PhonyG 当前读取五个运行时环境变量。未设置、格式错误或不符合有效范围时会使用默认值。

| 变量 | 类型 | 默认值 | 读取行为 |
|---|---|---:|---|
| `PHONYG_ADDR` | 字符串 | `:8080` | 去除首尾空白；空值使用默认监听地址。 |
| `PHONYG_DATA_DIR` | 路径 | `./data` | 保存 `phonyg.db` 与 `jwt_secret`。Docker 镜像默认覆盖为 `/data`。 |
| `PHONYG_JWT_SECRET` | 字符串 | 空 | 非空时直接使用；为空时读取或生成 `PHONYG_DATA_DIR/jwt_secret`。 |
| `PHONYG_MAX_BODY_BYTES` | 十进制整数 | `67108864` | 请求体上限，单位字节；解析失败或 `<= 0` 回退到 64 MiB。 |
| `PHONYG_JWT_TTL_HOURS` | 十进制整数 | `24` | 管理 JWT 有效小时数；解析失败或 `<= 0` 回退到 24。 |

## 示例

```bash
export PHONYG_ADDR=0.0.0.0:8080
export PHONYG_DATA_DIR=/srv/phonyg
export PHONYG_JWT_SECRET='replace-with-long-random-value'
export PHONYG_MAX_BODY_BYTES=134217728
export PHONYG_JWT_TTL_HOURS=12
./bin/phonyg
```

{: .important }
如果设置了 `PHONYG_JWT_SECRET`，多次重启或多副本必须保持相同值，否则现有管理员登录令牌会立即失效。不要把真实值提交进仓库。

## 数据文件

| 文件 | 用途 |
|---|---|
| `phonyg.db` | 管理员、渠道、模型、Key、预设、设置、请求元数据和统计。 |
| `phonyg.db-wal` | SQLite WAL；运行中可能包含尚未 checkpoint 回主库的数据。 |
| `phonyg.db-shm` | WAL 共享内存索引。 |
| `jwt_secret` | 仅在没有 `PHONYG_JWT_SECRET` 时创建。 |
