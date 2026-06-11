<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-worker</h1>

<p align="center">
  一个部署在 Cloudflare Workers 上的网站图标发现工具，支持在 Web UI 中预览候选图标，并以指定格式下载 —— 无需任何服务端图像处理。
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-GPL--3.0-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-JavaScript-F7DF1E.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-worker?color=success">
  <img alt="Deploy" src="https://github.com/chius-me/favicon-worker/actions/workflows/deploy.yml/badge.svg">
</p>

## 项目概览

`favicon-worker` 是 [favicon-fisher](https://github.com/chius-me/favicon-fisher) 的 Cloudflare Workers 重写版。只需输入 URL 或域名，它会自动补全 `https://`，拉取 HTML，解析 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon`、`mask-icon`、Web Manifest 图标以及兜底的 `/favicon.ico`，对候选图标评分排序后供用户预览和下载。

所有图像格式转换均在浏览器端通过 Canvas API 完成 —— Worker 不处理任何像素数据。

## 功能特性

- **Web UI 预览：** 输入 URL 即可在浏览器中预览发现的所有 favicon 候选
- **格式下载：** 支持将图标下载为 `png`、`jpg`、`webp` 或 `ico` 格式
- **SVG 透传：** 若源图标为 SVG，可直接下载 `svg` 格式
- **Web Manifest 发现：** 异步获取并解析 `manifest.json` 以发现更多图标
- **CORS 代理：** Worker 代理图标下载，绕过浏览器 CORS 限制
- **Canvas 转换：** 所有光栅格式转换在客户端完成（无需服务端图像库）
- **ICO 输出：** 通过将 PNG 包装在最小 ICO 头中生成合法 ICO 文件
- **零依赖部署：** Workers Static Assets —— API 与前端一体化部署

## 快速开始

安装依赖：

```bash
npm install
```

本地运行：

```bash
npx wrangler dev
```

然后打开：

```text
http://localhost:8787
```

部署到 Cloudflare：

```bash
npx wrangler deploy
```

## API

### `POST /api/preview`

发现指定 URL 的 favicon 候选。

请求：

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
      "sizes": null,
      "content_type": "image/svg+xml",
      "allowed_types": ["svg"]
    }
  ]
}
```

### `GET /api/proxy?url=<icon_url>`

代理图标下载以绕过 CORS。返回原始二进制数据及对应的 `Content-Type`。

## CI/CD

推送到 `main` 分支会通过 GitHub Actions 自动部署。

所需的 GitHub Secrets：

- `CLOUDFLARE_API_TOKEN` — 具有 Workers Scripts Edit 权限的 Cloudflare API Token
- `CLOUDFLARE_ACCOUNT_ID` — Cloudflare 账户 ID

## 项目结构

- `src/worker.js`：Worker 入口 —— API 路由和 favicon 发现逻辑
- `public/index.html`：前端 HTML
- `public/app.js`：前端 JS —— 预览渲染、Canvas 格式转换、下载
- `public/style.css`：前端样式
- `.github/workflows/deploy.yml`：GitHub Actions 部署工作流

## 补充说明

- SVG 输出为透传模式，仅在源图标为 SVG 时可用。
- ICO 输出通过 Canvas 将源图像转为 PNG，再包装在最小 ICO 容器中生成。
- 如需 CLI 和 Docker 版本，请参见 [favicon-fisher](https://github.com/chius-me/favicon-fisher)。

## 开源协议

本项目基于 [GNU General Public License v3.0](./LICENSE) 开源。
