<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

# favicon-worker

[favicon-fisher](../README.zh.md) 的 Cloudflare Workers 版本 —— 同一产品，边缘托管 UI，格式转换在浏览器完成。

```bash
npm install
npx wrangler secret put FVF_SIGNING_SECRET   # 生产环境设置一次
npx wrangler deploy

# 本地
npx wrangler dev   # http://localhost:8787
```

产品说明与其它运行方式见 **[根目录 README](../README.zh.md)**。
