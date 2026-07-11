<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-worker</h1>

favicon-fisher 的 Cloudflare Workers 实现 —— 零服务端图像处理，格式转换全部在浏览器 Canvas API 中完成。

## 部署

```bash
npm install
npx wrangler secret put FVF_SIGNING_SECRET
npx wrangler deploy
```

## 本地开发

```bash
npx wrangler dev
# 打开 http://localhost:8787
```

## 所需密钥

在 Cloudflare 控制台或 GitHub Actions secrets 中配置：

- `CLOUDFLARE_API_TOKEN` — 具备 Workers Scripts Edit 权限的 API Token
- `CLOUDFLARE_ACCOUNT_ID` — Cloudflare Account ID
- `FVF_SIGNING_SECRET` — `/api/proxy` token 的 HMAC 密钥（`wrangler secret put`）

可选：

- `FVF_ALLOW_PRIVATE=1` — 允许私网/环回目标（**切勿**在公网开启）

## 功能

- **Canvas 转换** — 下载为 `png`、`jpg`、`webp` 或 `ico`
- **SVG 直通** — 源为 SVG 时
- **Web Manifest 发现** — 抓取 `manifest.json` 中的图标
- **签名代理** — `GET /api/proxy?url=...&token=...`（token 来自 preview，非开放代理）
- **SSRF 防护、体积限制、限流、安全响应头**
- **Workers Static Assets** — API + 前端一次部署

## API

与 Go 版本一致：`POST /api/preview` 发现图标，`GET /api/proxy` 代理下载（需 token）。

详见[根目录 README](../README.zh.md) 的 API 与安全说明。
