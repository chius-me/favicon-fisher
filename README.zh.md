<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  发现、预览并下载任意网站的 favicon，支持 CLI、Go Web UI 和 Cloudflare Worker。
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
  <img alt="Container" src="https://github.com/chius-me/favicon-fisher/actions/workflows/container.yml/badge.svg">
</p>

## 项目简介

`favicon-fisher` 会发现网站的 favicon 候选项、排序，并下载你选择的图标。你可以在终端中使用它，也可以运行本地 Web 应用，或把 Worker 版本部署到 Cloudflare。

| 形态 | 说明 | 技术栈 |
|---|---|---|
| **`fvf`** | 命令行 favicon 发现与下载工具 | Go |
| **`fvf-web`** | 浏览器 Web UI + API | Go |
| **`favicon-worker`** | Cloudflare Workers 零运维部署 | TypeScript |

给定 URL 或裸域名后，应用会在需要时补全 `https://`，抓取页面，从 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon`、`mask-icon`、Web manifest 和 `/favicon.ico` 中解析候选图标，再按来源和尺寸排序。

## 环境要求

| 任务 | 要求 |
|---|---|
| 构建 CLI 或 Go Web UI | Go 1.26+ |
| 使用 Docker 运行 | Docker 和 Docker Compose v2 |
| 部署 Worker | Node.js 20+ 和 Wrangler |

## 快速开始

### CLI

```bash
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com
```

运行 `./fvf --help` 查看全部参数。

### Web UI

```bash
go build -o fvf-web ./cmd/fvf-web
PORT=8080 ./fvf-web
# 打开 http://localhost:8080
```

### Docker

```bash
docker compose up --build
# 打开 http://localhost:8080
```

Compose 文件名为 `docker-compose.yaml`，Docker Compose 会默认识别。

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler deploy
```

Worker 模式的完整文档请参阅 [`worker/`](./worker/) 目录。

## 功能特性

- **智能发现**：解析 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon`、`mask-icon`、Web manifest 以及 `/favicon.ico`
- **URL 规范化**：接受裸域名（如 `github.com`），自动补全 `https://`
- **候选排序**：按尺寸、类型和来源优先级自动选择最佳图标
- **批量下载**：CLI 可通过 `--all` 下载所有发现的候选图标
- **代理支持**：CLI 支持 `--proxy`，也会读取 `HTTP_PROXY` / `HTTPS_PROXY`
- **格式转换**：`fvf-web` 在源文件支持时可下载 **png**、**jpg**、**svg** 或 **ico**；Worker 浏览器代码支持 **png**、**jpg**、**webp**、**ico** 和 **svg** 直通
- **Worker 零服务端图像处理**：栅格格式转换在浏览器 Canvas API 上完成
- **跨平台发布**：在 [Releases 页面](https://github.com/chius-me/favicon-fisher/releases) 提供 Linux、macOS、Windows 预编译二进制

## CLI 使用

```bash
fvf [flags] <url>
```

| 参数 | 说明 |
|---|---|
| `-o, --out DIR` | 输出目录。默认：`./out` |
| `--json` | 输出结构化 JSON，适合脚本处理 |
| `--all` | 下载所有发现的 favicon 候选项，而不是只下载最佳结果 |
| `--proxy URL` | 使用 HTTP 代理，例如 `http://127.0.0.1:8080` |

示例：

```bash
# 下载最佳 favicon
./fvf --out tmp https://github.com

# 下载所有候选项
./fvf --all --out tmp https://go.dev

# 输出 JSON
./fvf --json --out tmp https://go.dev

# 使用显式代理
./fvf --proxy http://127.0.0.1:8080 https://example.com
```

省略 `<url>` 时，`fvf` 会进入交互式输入。JSON 模式必须提供 URL 参数。

## Go Web UI

运行本地服务：

```bash
go build -o fvf-web ./cmd/fvf-web
PORT=8080 ./fvf-web
```

打开 `http://localhost:8080`，输入 URL，预览候选图标并下载所选图标。服务会读取 `PORT`；未设置时监听 `8080`。

## Docker

构建并运行本地镜像：

```bash
docker compose up --build
```

如需换成本机其他端口，不需要改变容器内应用端口：

```bash
PORT=9090 docker compose up --build
# 打开 http://localhost:9090
```

停止服务：

```bash
docker compose down
```

直接运行已发布镜像：

```bash
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:latest
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:v1.0.1
```

镜像标签：`:latest` 表示默认分支，另有 `:main`、发布标签如 `:vX.Y.Z`、提交标签如 `:sha-<hash>`。

## API

Go Web UI 和 Worker 共用预览接口。下载流程不同，因为 Worker 在浏览器中完成转换。

下载与代理接口需要 preview 返回的短期 **HMAC token**（防止开放代理滥用）。

### `POST /api/preview`

发现指定 URL 的 favicon 候选（HTML link、Web Manifest、`/favicon.ico`）。无扩展名的 CDN 地址会在需要时探测 `Content-Type`。

```json
{
  "url": "https://github.com"
}
```

响应：

```json
{
  "input_url": "https://github.com",
  "page_url": "https://github.com",
  "recommended_icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "icons": [
    {
      "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
      "token": "<hmac-token>",
      "source_rel": "icon",
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `POST /api/download`（fvf-web）

下载指定格式的图标，必须携带 preview 返回的 `token`。响应体为二进制文件。

```json
{
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "format": "png",
  "token": "<hmac-token>"
}
```

### `GET /api/proxy?url=<icon_url>&token=<hmac-token>`（Worker）

通过 Worker 代理图标下载，帮助浏览器绕过远端 CORS。`token` 必填。

## 安全

`fvf-web` 与 Worker 会在服务端代发 HTTP 请求。公网部署默认启用以下控制：

| 控制 | 行为 |
|---|---|
| **SSRF 防护** | 仅允许 `http`/`https`；拦截私网、环回、链路本地、CGNAT 与元数据类目标；重定向逐跳校验 |
| **签名 token** | `/api/download` 与 `/api/proxy` 需要 preview 签发的 HMAC token（有效期 15 分钟） |
| **体积限制** | HTML ≤ 5 MiB，图标 ≤ 5 MiB，manifest ≤ 1 MiB，JSON 请求 ≤ 64 KiB |
| **限流** | 每个客户端 IP 每分钟约 60 次 API 请求（尽力而为） |
| **安全响应头** | `X-Content-Type-Options`、`CSP`、`X-Frame-Options`、`Referrer-Policy` |
| **错误脱敏** | 不向客户端泄露拨号/DNS 等内部细节 |

环境变量：

| 变量 | 适用 | 说明 |
|---|---|---|
| `FVF_SIGNING_SECRET` | Web + Worker | 下载/代理 token 的 HMAC 密钥；生产环境请设置稳定值 |
| `FVF_ALLOW_PRIVATE` | Web + Worker | 仅在受信任内网设为 `1`；**切勿**在公网开启 |
| `PORT` | Web | 监听端口（默认 `8080`） |

**CLI** 允许访问本机/内网目标（方便本地开发），但仍强制 `http`/`https`、User-Agent、超时与体积上限。

## Cloudflare Worker

Worker 版本提供相同风格的 UI、静态资源和 TypeScript Worker API。它不在服务端运行 Go 图像转换。浏览器代码通过 `/api/proxy` 获取图标字节，再用 Canvas 转换栅格格式。

```bash
cd worker
npm install
npx wrangler secret put FVF_SIGNING_SECRET
npx wrangler deploy
```

Worker 专用命令和部署说明见 [`worker/README.zh.md`](./worker/README.zh.md)。

## 开发

构建两个 Go 入口：

```bash
go build ./cmd/fvf ./cmd/fvf-web
```

运行测试：

```bash
go test ./...
go test -race ./...
```

运行 Worker 检查：

```bash
cd worker
npm install
npm run check
npm run build
```

共享发现夹具位于 `testdata/golden/`。

## 项目结构

| 路径 | 说明 |
|---|---|
| `cmd/fvf/` | CLI 入口 |
| `cmd/fvf-web/` | Web 服务入口 |
| `internal/fetcher/` | 核心发现、排序与下载逻辑 |
| `internal/convert/` | 格式转换（Go 图像处理） |
| `internal/security/` | SSRF、签名、限流、响应头、体积限制 |
| `internal/web/` | API 处理器与内嵌静态资源 |
| `worker/` | Cloudflare Workers 变体（TypeScript，零服务端处理） |
| `testdata/golden/` | 共享 HTML/manifest 发现测试夹具 |

## 补充说明

- SVG 输出为直通模式，要求源图标为 SVG。
- `fvf-web` 的 ICO 输出为直通模式，要求源图标为 ICO。
- Worker 版在浏览器中将 PNG 字节包装为最小 ICO 容器来生成 ICO 输出。
- CLI 和 Web 请求对慢速服务器使用 15 秒超时。
- Docker 镜像以非 root（`65532`）运行；Compose 启用只读根文件系统并挂载 `/tmp` tmpfs。

## 许可证

[GNU GPL v3.0](./LICENSE)
