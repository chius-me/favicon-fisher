<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  发现、预览并下载任意网站的 favicon — 支持 CLI、Web UI、Cloudflare Worker 三种形态。
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-MIT-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
  <img alt="Container" src="https://github.com/chius-me/favicon-fisher/actions/workflows/container.yml/badge.svg">
</p>

## 项目简介

`favicon-fisher` 是一个轻量级工具，用于发现网站的 favicon、预览候选图标并下载精选结果。提供三种使用方式：

| 形态 | 说明 | 技术栈 |
|---|---|---|
| **`fvf`** | 命令行工具 — favicon 发现与下载 | Go |
| **`fvf-web`** | 浏览器 Web UI + API | Go |
| **`favicon-worker`** | Cloudflare Workers 零运维部署 | TypeScript |

给定一个 URL 或域名，它会自动补全 `https://`，抓取 HTML，从 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon`、Web manifest 以及兜底的 `/favicon.ico` 中解析候选图标，按相关性排序，让你预览并选择格式下载。

## 快速开始

### CLI

```bash
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com
```

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

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler deploy
```

Worker 模式的完整文档请参阅 [`worker/`](./worker/) 目录。

## 功能特性

- **智能发现** — 解析 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon`、`mask-icon`、Web manifest 以及 `/favicon.ico`
- **URL 规范化** — 接受裸域名（如 `github.com`），自动补全 `https://`
- **候选排序** — 按尺寸、类型和来源优先级自动选择最佳图标
- **格式转换** — 支持下载为 **png**、**jpg**、**webp** 或 **ico**（Worker）/ **png**、**jpg** 或 **svg**（fvf-web）
- **零服务端图像处理**（Worker）— 所有像素转换在浏览器 Canvas API 上完成
- **跨平台发布** — 在 [Releases 页面](https://github.com/chius-me/favicon-fisher/releases) 提供 Linux、macOS、Windows 预编译二进制

## API

`fvf-web` 和 `favicon-worker` 暴露相同的 API 接口。

### `POST /api/preview`

发现指定 URL 的 favicon 候选列表。

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
      "source_rel": "icon",
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `POST /api/download`（fvf-web）

下载所选格式的图标。返回二进制文件。

```json
{
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "format": "png"
}
```

### `GET /api/proxy?url=<icon_url>`（Worker）

代理图标下载以绕过浏览器的 CORS 限制。

## CLI 使用

```bash
fvf [--out DIR] [--json] <url>
```

| 参数 | 说明 |
|---|---|
| `--out DIR` | 输出目录（默认为当前目录） |
| `--json` | 输出结构化 JSON，适合脚本处理 |

示例：

```bash
# 基础用法
./fvf --out tmp https://github.com

# JSON 模式
./fvf --json --out tmp https://go.dev
```

## 容器镜像

多架构镜像（`linux/amd64` + `linux/arm64`）已发布至 GHCR：

```bash
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:latest
docker run --rm -p 8080:8080 ghcr.io/chius-me/favicon-fisher:v1.0.1
```

标签：`:latest`（默认分支）、`:main`、`:vX.Y.Z`（发布版本）、`:sha-<hash>`。

## 测试

```bash
go test ./...
```

## 项目结构

| 路径 | 说明 |
|---|---|
| `cmd/fvf/` | CLI 入口 |
| `cmd/fvf-web/` | Web 服务入口 |
| `internal/fetcher/` | 核心发现、排序与下载逻辑 |
| `internal/convert/` | 格式转换（Go 图像处理） |
| `internal/web/` | API 处理器与内嵌静态资源 |
| `worker/` | Cloudflare Workers 变体（TypeScript，零服务端处理） |

## 补充说明

- SVG 输出为直通模式，仅当源图标为 SVG 时才可用。
- Worker 版通过将 PNG 包装在最小 ICO 容器中实现 ICO 输出。
- CLI 对慢速服务器设置了 10 秒超时。

## 许可证

[MIT](./LICENSE)
