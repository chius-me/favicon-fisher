<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  发现并下载任意网站的最佳 favicon —— 支持 CLI、本地 Web UI 与 Cloudflare Worker。
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
</p>

## 是什么

输入网址（或裸域名）。**favicon-fisher** 会发现页面上的图标候选、排序，并帮你下载想要的那一个。

会检查常见来源：`<link rel="icon">`、apple-touch 图标、mask-icon、Web Manifest，以及经典的 `/favicon.ico`。

## 怎么用

| | | |
|---|---|---|
| **CLI**（`fvf`） | 终端 / 脚本 | 最适合命令行与自动化 |
| **Web UI**（`fvf-web`） | 本机浏览器或 Docker | 预览候选、选格式、下载 |
| **Worker** | Cloudflare 边缘 | 同样能力，几乎零运维 |

## 快速开始

### CLI

```bash
# 源码构建
go build -o fvf ./cmd/fvf
./fvf --out tmp https://github.com

# 也可从 Releases 下载预编译二进制
```

常用参数：`--all`（全部候选）、`--json`（机器可读）、`--proxy`、`-o`（输出目录）。  
完整说明见 `./fvf --help`。

### Web UI

```bash
go build -o fvf-web ./cmd/fvf-web
./fvf-web
# 打开 http://localhost:8080
```

或用 Docker：

```bash
docker compose up --build
# 打开 http://localhost:8080
```

镜像：`ghcr.io/chius-me/favicon-fisher:latest`（亦有 `:vX.Y.Z`）。

### Cloudflare Worker

```bash
cd worker
npm install
npx wrangler secret put FVF_SIGNING_SECRET   # 首次
npx wrangler deploy
```

## 亮点

- **智能发现** — HTML 图标、Manifest、favicon 回退  
- **合理排序** — 优先标准 icon 与更大尺寸  
- **多种格式** — PNG / JPG / SVG / ICO（Worker 浏览器侧还支持 WebP）  
- **便于脚本** — CLI 支持 JSON 与批量下载  
- **开箱可发** — 多平台 Release、Docker 镜像、Worker 一键部署  

## 环境要求

| 目标 | 需要 |
|---|---|
| 构建 CLI 或 Web UI | Go 1.26+ |
| Docker 运行 | Docker Compose v2 |
| 部署 Worker | Node.js 20+ 与 Wrangler |

## 说明

- 定位是**本地工具**，或需谨慎加固后再公网部署。Web / Worker 会代你抓取远程页面 —— 生产环境请设置 `FVF_SIGNING_SECRET`（Worker **强制要求**；未配置 ≥32 字符密钥时 fail-closed），且勿在公网放开内网访问。  
- 图标预览一律走同源签名 `/api/proxy`，浏览器不会直接加载远端（或内网）图标 URL。  
- Go 的 SSRF 在拨号时复验 DNS；Worker 拦截私网 IP 字面量/主机名，并依赖 Cloudflare 出口限制（与 Go 拨号器并非完全等价）。  
- SVG（以及 Go Web UI 的 ICO）仅在内容确实匹配格式时直通，不会仅信任扩展名。  
- 反向代理后请设置 `FVF_TRUSTED_PROXIES`，否则限流可能变成「全站共享」。  
- 安全策略：[SECURITY.md](./SECURITY.md)。API / 部署 / 架构：[docs/](./docs/)。  
- 贡献与 CI 门禁：[CONTRIBUTING.md](./CONTRIBUTING.md)。  
- 更多细节：`./fvf --help`，以及源码目录 `cmd/`、`internal/`、`worker/`。

## 许可证

[GNU GPL v3.0](./LICENSE)
