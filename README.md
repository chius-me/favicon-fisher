# favicon-fisher

一个简单的 Web 工具：输入网站地址，返回该网站正在使用的 favicon / icon。

当前是一个可运行的 MVP，重点先把抓取逻辑和结果展示跑通。

## 功能

- 输入网站 URL 并抓取图标
- 自动补全 `https://`
- 解析 HTML 中的 `rel="icon"`
- 解析 `apple-touch-icon`
- 解析 `manifest` 中声明的 icons
- 相对路径自动转绝对路径
- 回退到 `/favicon.ico`
- 按尺寸和来源排序，选出 best icon
- 提供页面展示和 JSON API

## 技术栈

- Next.js 15
- React 19
- TypeScript
- Cheerio
- Vitest

## 本地开发

```bash
npm install
npm run dev
```

默认启动后访问：

```bash
http://localhost:3000
```

## 构建与验证

```bash
npm run lint
npm test
npm run build
```

## API

```bash
GET /api/favicon?url=example.com
```

示例响应：

```json
{
  "siteUrl": "https://example.com/",
  "title": "Example Domain",
  "bestIcon": {
    "url": "https://example.com/apple-touch-icon.png",
    "rel": "apple-touch-icon",
    "sizes": "180x180",
    "size": 180,
    "source": "html"
  },
  "candidates": [
    {
      "url": "https://example.com/apple-touch-icon.png",
      "rel": "apple-touch-icon",
      "sizes": "180x180",
      "size": 180,
      "source": "html"
    }
  ]
}
```

## 当前实现说明

抓取顺序大致是：

1. 请求目标网页 HTML
2. 提取页面里的 icon link
3. 如果有 manifest，则继续解析 manifest icons
4. 补一个 `/favicon.ico` 作为兜底
5. 对候选图标排序并返回最优项

## 后续可以继续做

- 下载图标
- 复制图标 URL
- 展示更多站点元信息
- 更细致的错误提示
- 请求超时与重试
- 更丰富的图标质量评分策略
- 部署到 Cloudflare Pages / Vercel
