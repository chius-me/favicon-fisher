<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-worker</h1>

favicon-fisher 的 Cloudflare Workers 实现版 — 服务端零图像处理，所有格式转换在浏览器 Canvas API 中完成。

## 部署

```bash
npm install
npx wrangler deploy
```

## 本地开发

```bash
npx wrangler dev
# 打开 http://localhost:8787
```

## 所需密钥

在 Cloudflare 面板或 GitHub Actions Secrets 中设置：

- `CLOUDFLARE_API_TOKEN` — 具有 Workers Scripts Edit 权限的 API Token
- `CLOUDFLARE_ACCOUNT_ID` — 您的 Cloudflare 账户 ID

## 功能特性

- **Canvas 转换** — 下载为 `png`、`jpg`、`webp` 或 `ico`
- **SVG 直通** — 当源图标为 SVG 时直接输出
- **Web manifest 发现** — 异步抓取 `manifest.json` 中的图标
- **CORS 代理** — `GET /api/proxy?url=...` 绕过浏览器 CORS 限制
- **Workers Static Assets** — 一次部署搞定 API + 前端

## API

与 Go 版一致：`POST /api/preview` 用于发现图标，`GET /api/proxy` 用于代理下载。

API 详细信息请参阅[根目录 README](../README.zh.md)。
