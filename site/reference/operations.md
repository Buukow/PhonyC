---
layout: default
title: 升级、备份与回滚
parent: 配置参考
nav_order: 3
permalink: /reference/operations/
---

# 升级、备份与回滚

## 升级前检查

1. 记录当前镜像标签或二进制提交。
2. 检查 `/api/health`。
3. 备份数据目录。
4. 保留旧容器或旧二进制，直到代理与管理台验证完成。

## SQLite 安全备份

最简单的方法是短暂停止 PhonyG 后复制整个数据目录：

```bash
docker stop phonyg
docker run --rm -v phonyg-data:/data:ro -v "$PWD":/backup \
  alpine sh -c 'tar czf /backup/phonyg-$(date +%Y%m%d-%H%M%S).tgz -C /data .'
docker start phonyg
```

需要不停机时，使用 SQLite backup API 或 `sqlite3 .backup`，不要只复制 `phonyg.db` 而忽略 WAL。

## 回滚容器

```bash
docker stop phonyg
docker rm phonyg
docker rename phonyg-backup phonyg
docker start phonyg
curl --fail http://127.0.0.1:8080/api/health
```

如果数据库迁移后旧版本不兼容，先恢复升级前的数据卷备份，再启动旧镜像。

## 本地二进制回滚

将旧二进制和数据备份分别保存。systemd 部署可在替换二进制前复制：

```bash
cp bin/phonyg bin/phonyg.rollback
cp -a data "data.rollback.$(date +%s)"
```

回滚时停止服务，恢复二进制和数据，再启动并检查健康接口。
