# PhonyG GitHub Pages + Jekyll 文档站设计

## 目标

为 PhonyG 建立完整的中文静态文档站，发布地址为
`https://buukow.github.io/PhonyC/`。站点采用 GitHub Pages + Jekyll，使用成熟的
Just the Docs 信息架构和 PhonyG 暖灰绿色视觉定制，面向首次部署、日常运维、
客户端伪装和请求捕获验证。

文档不能只用文字解释：所有关键管理台能力都配有来自当前 `23346` 实例的无头
Chromium 截图。截图直接保留页面内容，不做脱敏；请求捕获教程的第三张配图由用户
提供，展示 AI 客户端收到 `captured` 的成功响应。

## 信息架构

侧栏和页面顺序如下：

1. 首页
2. 快速开始
3. 本地构建
4. Docker 部署
5. 功能指南
   - 概览与统计
   - 渠道与模型映射
   - 用户 Key 与伪装模式
   - 客户端预设
   - 请求捕获
   - 请求日志
   - 自动测活与增强模式
   - 自动重试
6. 配置参考
   - 环境变量
   - API 路径与协议
   - 升级、备份与回滚
   - 故障排查

“Docker 部署”必须紧跟“本地构建”之后。Docker 页面覆盖 GHCR 镜像、`docker run`、
Compose、数据卷、全部环境变量、升级、备份和回滚；本地构建页面覆盖前端嵌入、Go
编译、运行参数和开发循环。

## 技术方案

### Jekyll 站点

- 源码放在仓库的 `site/`，避免与现有内部 `docs/` 设计文档混用。
- `site/Gemfile` 锁定 Jekyll、Just the Docs 和 `jekyll-remote-theme` 的兼容版本，
  `site/Gemfile.lock` 一并提交，确保本地与 Actions 使用相同依赖。
- `_config.yml` 设置 `url: https://buukow.github.io`、`baseurl: /PhonyC`、中文语言和
  Just the Docs remote theme，并在 `plugins` 中启用 `jekyll-remote-theme`。
- 自定义 `_layouts`、`_includes` 和 `assets/css/custom.scss`，保留标准侧栏、搜索、
  目录、代码块、提示框和响应式导航，同时使用 PhonyG 的暖灰绿色配色。
- 页面正文使用 Markdown；截图统一存放在 `site/assets/images/`，所有链接使用
  `relative_url` 或 `absolute_url`，确保项目页 baseurl 正确。
- `_config.yml` 的 `exclude` 排除内部规格、构建缓存和临时截图数据。

### GitHub Pages 发布

新增 `.github/workflows/pages.yml`：

1. 在 `main` 分支 push 或手动触发时 checkout。
2. 使用 Ruby、Bundler，在 `site/` 工作目录执行 `bundle config path vendor/bundle` 和
   `bundle install`，安装锁定的 Jekyll、Just the Docs 与 remote-theme 依赖。
3. 从仓库根目录执行
   `bundle exec jekyll build --source site --destination site/_site --baseurl /PhonyC`
   （Bundler 工作目录/`BUNDLE_GEMFILE` 指向 `site/Gemfile`）。
4. 上传 `_site` artifact。
5. 使用 `actions/deploy-pages` 发布到 GitHub Pages。

README 增加文档站链接和本地预览命令。工作流权限仅开放 `contents: read`、
`pages: write`、`id-token: write`，并配置 `concurrency` 避免并行发布覆盖。
仓库的 GitHub Pages 设置需选择 **GitHub Actions** 作为 Source；工作流使用
`actions/configure-pages` 初始化 Pages 元数据，然后上传 `site/_site`。

## 内容范围

### 首页与快速开始

首页用一句话说明 Header 重组、Body 玻璃穿透、协议适配、Key 伪装和 SQLite 单机
管理台，并提供本地构建、Docker、管理台初始化和 API 调用入口。快速开始说明首次
管理员初始化、默认端口、健康检查和一个最小 OpenAI/Anthropic 请求。

### 本地构建与 Docker 部署

本地构建严格以 Makefile、Dockerfile 和 `internal/config/config.go` 为准，说明 Go、
Node.js/npm、`make build`、前端构建、`PHONYG_ADDR` 与 `PHONYG_DATA_DIR`。

Docker 页面说明：

- `ghcr.io/buukow/phonyg:1.9` 与 `latest` 标签策略；
- 端口映射、`/data` 卷和容器用户；
- `docker run` 和 `docker compose` 示例；
- 固定版本升级、数据库备份、回滚和健康检查；
- 所有运行时环境变量：
  `PHONYG_ADDR`、`PHONYG_DATA_DIR`、`PHONYG_JWT_SECRET`、
  `PHONYG_MAX_BODY_BYTES`、`PHONYG_JWT_TTL_HOURS`，包括默认值、类型、作用和示例。

### 功能指南

每页遵循“用途 → 操作步骤 → 实景截图 → 行为细节 → 注意事项 → 相关 API/配置”的
模板。

- **渠道与模型映射**：OpenAI/Anthropic 协议、Base URL、上游 Key、优先级、超时、
  额外 Header、模型映射、`/v1/models` 聚合和临时禁用。
- **用户 Key 与伪装模式**：passthrough、preset、custom，Key 生命周期和绑定关系。
- **客户端预设**：四个内置 Codex/Claude 预设、结构化 JSON、生成器、模板引用、
  保护 Header、父子字段继承，以及左侧“缺失补全”和右侧默认“强制覆盖”的语义。
- **自动测活与增强模式**：固定提问词、间隔和随机偏移、状态码临时禁用/恢复、
  增强词库、随机提示词、stream-first 与非流式 fallback。
- **自动重试**：触发状态码、次数、与渠道选路和测活状态的关系。
- **请求日志**：筛选、状态、耗时、Token、渠道、错误摘要和捕获请求的字段语义。

### 请求捕获深度教程

独立页面以四步图文流程呈现：

1. 在管理台开启捕获并取得系统固定 API Key。
2. 将 Codex、Claude Code 或其他客户端 Base URL 指向网关，并使用捕获 Key 发起一次请求。
3. 客户端收到协议适配后的 `captured` 成功响应；用户提供的第三张截图展示该结果。
4. 在管理台查看捕获到的过滤后完整业务 Header，一键保存或覆盖为客户端预设。

页面必须明确：布防中只捕获、不转发、不访问上游；只捕获下一次请求，完成后要重新
布防；未布防时固定 Key 返回 403；鉴权、Content-Length、Accept-Encoding、连接和
其他 hop-by-hop/传输头被过滤；会话级 `Session-Id`、`Thread-Id`、Claude session
等业务头保留；捕获请求仍写入 PhonyG 请求日志，状态为 200、无渠道、零 Token、
空错误摘要，且 `capture-only` 不参与上游统计。

页面同时说明当前捕获存储使用单值 Header 映射：同名 Header 出现多个值时只保留
第一个值。因此“完整”指过滤范围内的业务 Header 名和值，不代表保留重复 Header 的
全部多值实例。

核心配图为：

1. 23346 请求捕获管理台页面；
2. 23346 捕获到的完整 Header 表；
3. 用户提供的 AI 客户端 `captured` 成功响应图，固定保存为
   `site/assets/images/capture-client-response.png`。

## 截图生产

新增 `scripts/capture-doc-screenshots.mjs`，使用 Playwright/Chromium headless：

- 在仓库根目录新增仅用于文档自动化的 `package.json`/npm script，锁定 Playwright
  版本并提交 lockfile；本地首次运行执行 `npm ci` 和
  `npx playwright install chromium`，CI 如需重拍截图则使用
  `npx playwright install --with-deps chromium`；
- 使用独立临时浏览器上下文，不读取 Codex/Claude 全局配置；
- 访问 `http://127.0.0.1:23346`，完成登录后只进行页面浏览、展开和必要的 UI 切换；
- 截取 Dashboard、Channels、Keys、Presets（树编辑和开关）、Settings（自动测活增强）、
  Capture 和 Logs 页面；
- 等待网络空闲和关键元素出现后再截图，固定桌面宽度并生成 PNG；
- 在脚本输出中记录截图文件、页面 URL 和时间，失败时以非零状态退出；
- 只允许导航、滚动、展开折叠、切换前端编辑标签等只读交互；不点击保存、删除、测活、
  布防、重新布防、生成/刷新或其他会写入状态/触发请求的操作。若某截图必须改变临时
  表单状态，刷新页面后必须验证后端数据未改变；
- 不修改 23346 的持久化设置，不触发上游请求，不执行捕获请求；
- 用户提供的客户端响应图以固定文件名放入 `site/assets/images/capture-client-response.png`。

真实登录凭证不写入仓库，脚本通过环境变量读取：`PHONYG_DOCS_URL`、
`PHONYG_DOCS_USERNAME`、`PHONYG_DOCS_PASSWORD`。截图文件进入 Git，文档不承诺隐藏
页面中已存在的值，因为用户已明确要求不打码。脚本和错误日志不得打印用户名、密码、
JWT、用户 Key 或页面中的凭证字段。

## 验证标准

- `BUNDLE_GEMFILE=site/Gemfile bundle exec jekyll build --source site --destination site/_site --baseurl /PhonyC`
  成功且无 Liquid、链接或 Sass 错误。
- 本地 `BUNDLE_GEMFILE=site/Gemfile bundle exec jekyll serve --source site --host 0.0.0.0 --port 23342 --baseurl /PhonyC`
  可访问，导航、搜索、
  图片和代码复制在 `/PhonyC/` baseurl 下正常。
- Playwright 脚本成功生成全部管理台截图，图片尺寸和文件名稳定。
- 仓库根目录执行 `npm ci`，再以所需 `PHONYG_DOCS_*` 环境变量运行
  `npm run docs:screenshots`，脚本成功且不输出凭证。
- `site/assets/images/capture-client-response.png` 必须存在、可解码、达到正文可读尺寸，
  并在请求捕获页面通过 baseurl-safe 链接实际渲染；在用户提供前使用明确标注的占位图，
  Pages 发布验收前替换为真实截图。
- `cd web && npm test -- --run`、`cd web && npm run build`、
  `go test ./... -count=1` 继续通过。
- GitHub Actions Pages 工作流成功，最终 URL 为 `https://buukow.github.io/PhonyC/`。
- README 链接、Docker 命令、环境变量表和页面截图路径均可复核。

## 不在本次范围

- 不改动 PhonyG 后端 API、数据库 schema 或管理台行为。
- 不为文档站增加动态 API、评论、统计或第三方 SaaS。
- 不把当前工作区的内部规格文档自动发布到公开站点。
