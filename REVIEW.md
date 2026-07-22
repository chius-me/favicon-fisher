# favicon-fisher 全面 Review

## 总结结论

**综合评分：6.8 / 10**

这是一个定位清晰、工程基础不错的项目：CLI、Go Web、Cloudflare Worker 三种运行形态完整，安全意识明显高于普通个人项目，CI、跨平台 Release、容器发布也已经成型。当前 `main` 最新代码是 2026 年 7 月 11 日的 `d4f5e56`。

但它目前还不适合直接作为**无人值守的公网服务**。最需要处理的是：

1. Worker 使用公开的默认签名密钥。
2. Go Web 前端直接加载候选图标 URL，形成客户端内网请求入口。
3. 图像解码缺少像素限制，存在内存/CPU DoS。
4. 候选数量和网络探测次数没有上限。
5. Go 与 Worker 两套实现已经产生明显行为漂移。

> 本次是基于 GitHub `main` 分支的静态代码审查。我检查了核心 Go、Worker、前端、测试、Docker 和 GitHub Actions；由于执行环境无法拉取仓库，没有实际运行 `go test`、Worker 测试或攻击 PoC。

---

## 评分表

| 维度          |      评分 | 评价                          |
| ----------- | ------: | --------------------------- |
| 产品定位        | **8.0** | 使用场景清晰，三种运行方式有价值            |
| 架构设计        | **7.0** | 分层合理，但 Go/Worker 重复实现成本较高   |
| Go 代码质量     | **7.5** | 结构清楚，错误处理总体规范               |
| Worker 代码质量 | **5.5** | 单文件过大，安全回退和解析方式有问题          |
| 安全性         | **5.5** | 防御体系完整，但存在几个关键闭环缺口          |
| 性能与稳定性      | **6.0** | 有体积限制和超时，但缺少候选、像素及并发控制      |
| 测试          | **5.5** | Go 主路径覆盖尚可，Worker 基本没有运行时测试 |
| CI/CD       | **7.0** | 发布流程成熟，但供应链和发布门禁仍不足         |
| 文档          | **6.5** | 产品 README 简洁，但生产部署文档过薄      |

---

# P1：优先修复的问题

## 1. Worker 的默认签名密钥是公开常量

**严重程度：高**

Worker 未配置 `FVF_SIGNING_SECRET` 时，会使用：

```ts
'dev-only-change-me-fvf-signing-secret'
```

仓库是公开的，因此任何人都可以用这个字符串自行计算合法 HMAC token，绕过 preview 流程，直接调用 `/api/proxy` 抓取任意允许的公网 URL。

与此同时，自动部署 workflow 只是通过注释提醒设置 secret，并不会验证 secret 是否存在，所以这个不安全默认值很容易被直接部署到生产。

### 建议修复

Worker 应当 **fail-closed**：

```ts
async function signingKey(env: Env): Promise<CryptoKey> {
  if (!env.FVF_SIGNING_SECRET || env.FVF_SIGNING_SECRET.length < 32) {
    throw new Error('FVF_SIGNING_SECRET is not configured');
  }

  return crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(env.FVF_SIGNING_SECRET),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign', 'verify'],
  );
}
```

同时在部署 workflow 增加显式检查，或者通过 Cloudflare API/部署环境管理 secret。不要在源码中保留任何可用于生产签名的默认值。

---

## 2. Go Web 存在客户端侧内网请求入口

**严重程度：高**

攻击链是成立的：

1. 用户输入一个攻击者控制的网站。
2. 该网站返回：

```html
<link rel="icon" href="http://192.168.1.1/some-path">
```

3. 后端对该地址的 Content-Type 探测会被 SSRF 防护拦截，但候选项不会被删除。
4. `allowedTypesFor()` 的最后分支无条件返回 `["png", "jpg"]`，因此所谓的“不可预览候选过滤”实际上永远不会失败。
5. 前端直接执行：

```ts
heroIconEl.src = selectedIcon.icon_url;
img.src = icon.icon_url;
```

从而让**用户浏览器**访问该内网地址。

这不仅会泄露用户公网 IP 给各图标 CDN，还可能成为对路由器、NAS、本地管理页面等地址发起 GET 请求的客户端请求工具。

### 建议修复

Go Web 不应把远端 URL 直接设置为 `<img src>`。

应增加同源预览接口，例如：

```text
GET /api/proxy?url=...&token=...
```

前端统一使用：

```ts
img.src = `/api/proxy?url=${encodeURIComponent(icon.icon_url)}&token=${encodeURIComponent(icon.token)}`;
```

这也是 Worker 前端当前采用的方式。

修复后 CSP 可从：

```text
img-src 'self' data: https: http:
```

收紧为：

```text
img-src 'self' data: blob:
```

---

## 3. 图像解码炸弹可导致服务 OOM

**严重程度：高**

服务仅限制远端图标的压缩后字节数为 5 MiB，但随后直接调用 `image.Decode()`。一个压缩体积很小、像素尺寸极大的 PNG/GIF/JPEG，仍然可能导致数百 MiB 甚至数 GiB 的内存分配。

Worker 浏览器端也有类似问题：

```ts
const size = Math.max(img.naturalWidth, img.naturalHeight, 16);
canvas.width = size;
canvas.height = size;
```

没有上限，可能让用户浏览器创建超大 Canvas，造成卡死或崩溃。

### 建议修复

Go 在 Decode 前先检查：

```go
config, _, err := image.DecodeConfig(bytes.NewReader(data))
if err != nil {
    return Result{}, fmt.Errorf("decode source config: %w", err)
}

const (
    maxDimension = 4096
    maxPixels    = 16_000_000
)

pixels := int64(config.Width) * int64(config.Height)
if config.Width <= 0 ||
    config.Height <= 0 ||
    config.Width > maxDimension ||
    config.Height > maxDimension ||
    pixels > maxPixels {
    return Result{}, fmt.Errorf("image dimensions exceed limits")
}
```

Worker 端应把 Canvas 限制在 512 或 1024 像素，并在转换前检查 `naturalWidth × naturalHeight`。

---

## 4. 候选数量和探测请求没有上限

**严重程度：高，主要影响可用性**

Go 会逐个候选探测 Content-Type，且每个候选可能执行一次 HEAD，再执行一次 GET。候选数完全由远端 HTML 和 Manifest 控制，并且当前是串行处理。

Worker 同样会逐个探测扩展名不明确的候选。

Cloudflare Workers 免费计划单次 invocation 的 subrequest 上限为 50；每次重定向也会计入 subrequest，并且单个 subrequest 没有固定的时间上限。当前实现很容易被包含大量 `<link rel="icon">` 的页面打到平台限制。([Cloudflare Docs][1])

### 建议修复

建议设定：

```text
HTML icon candidates: 最多 32
Manifest icons: 最多 32
最终候选总数: 最多 48
Content-Type probes: 最多 12
Redirect hops: 最多 5
```

并对探测设置整体 deadline，而不是每个请求各自拥有完整超时时间。

Go 可以使用有限并发：

```go
sem := make(chan struct{}, 4)
```

Worker 建议使用 `AbortSignal.timeout()`，并避免无界的串行 subrequest。

---

# P2：重要的功能与架构问题

## 5. `--all` 会静默覆盖同名文件

**严重程度：中高**

`fetchAll` 按候选 URL 去重，但输出文件名是：

```text
hostname + sizeHint 或 relHint + extension
```

同一网站经常会有多个：

```html
<link rel="icon" href="/a.png">
<link rel="icon" href="/b.png">
```

两者都没有 size，并且 rel 都是 `icon`，最终都会写成类似：

```text
example.com.png
```

后一个文件覆盖前一个，但 `AllIcons` 中仍然会有多条记录指向这个路径。

### 修复方案

文件名加入：

* 候选序号；
* URL path basename；
* URL 的短哈希。

例如：

```text
example.com-01-favicon-32-a3f91c.png
example.com-02-icon-bce021.png
```

写文件时最好使用 `O_EXCL` 或显式冲突处理，避免静默覆盖。

---

## 6. 最佳候选选中后失败，不会尝试次优候选

**严重程度：中**

`BestCandidate()` 仅根据 rel priority 和 sizes 排序，没有验证图标是否真实存在或可解码。非 `--all` 模式直接下载第一个候选；如果首选 URL 返回 404，CLI 整体失败，而不会尝试第二名或 `/favicon.ico`。

这会显著影响真实网站兼容性，因为不少网站保留了失效的旧 favicon 声明。

### 建议

将“排名”与“选择”分离：

```text
排序候选 → 依次尝试 → 首个下载成功且可识别为图像的候选成为最佳结果
```

Preview API 可保留完整候选列表，但 `recommended_icon_url` 应指向已验证可访问的候选。

---

## 7. Go Web 的候选过滤函数逻辑错误

`allowedTypesFor()` 最后的代码是：

```go
return []string{"png", "jpg"}
```

所以不论 URL 和 Content-Type 是什么，永远都至少返回两个格式。

这导致：

* `len(allowed) == 0` 分支成为死代码；
* HTML、JSON、文本文件也会被列为可转换图标；
* 前端可能显示大量失败的候选；
* 前述客户端内网请求链成立。

应当在未知类型时返回空数组，或者先实际读取少量字节，通过 magic bytes 检测格式。

---

## 8. Go 与 Worker 已经出现行为漂移

**严重程度：中**

Go 使用 `x/net/html` 正式解析 HTML。

Worker 使用正则：

```ts
const linkRe = /<link\b[^>]*>/gi;
const re = new RegExp(`${name}\\s*=\\s*["']([^"']*)["']`, 'i');
```

这会漏掉：

```html
<link rel=icon href=/favicon.ico>
```

以及部分合法但格式复杂的 HTML。

其他漂移还包括：

| 行为              | Go                  | Worker                      |
| --------------- | ------------------- | --------------------------- |
| HTML 解析         | HTML parser         | Regex                       |
| Content-Type 探测 | HEAD + GET fallback | 仅 HEAD                      |
| 图片预览            | 直接远端 URL            | 同源 proxy                    |
| ICO 生成          | 直通                  | 浏览器包装 PNG                   |
| 格式支持            | PNG/JPG、SVG/ICO 直通  | PNG/JPG/WebP/ICO            |
| SSRF DNS 验证     | 解析并在拨号时复验           | 只检查 hostname 文本和 IP literal |

所谓的共享 golden fixture，目前只被 Go 测试消费，并没有 Worker parity test。

### 建议架构

不要继续手工维护两套规则。

可采用：

1. 建立语言无关的 JSON fixtures；
2. fixtures 包含输入 HTML、Manifest、期望候选和排序；
3. Go test 与 Worker Vitest 同时读取；
4. API DTO 生成 JSON Schema；
5. CI 对两边结果做 contract test。

---

## 9. Worker SSRF 防护的声明强于实际实现

Worker 的 `assertSafeUrl()` 会拦截：

* localhost；
* `.local`、`.internal`；
* 私网 IP literal；
* 部分保留地址。

但它不会解析普通 hostname 的 A/AAAA 结果。因此像“公网域名解析到保留地址”的情况，在应用代码层并未验证。

默认 Cloudflare Worker 对很多不可访问地址存在平台限制，但项目此时是在依赖 Cloudflare 平台行为，而不是实现完整的 DNS 级 SSRF 检查。Cloudflare 文档也明确表明 Worker `fetch()` 使用 Cloudflare DNS resolver，并且通过 hostname 发起请求与直接 IP URL 的行为不同。([Cloudflare Docs][2])

建议在文档中明确：

> Worker SSRF 防护包含应用层 URL literal 检查，并依赖 Cloudflare 的网络出口限制；它不提供与 Go 拨号器完全等价的 DNS 解析验证。

不要声称两者提供完全相同的 SSRF 保证。

---

## 10. 反向代理后 Go 限流可能变成“全站共享 60 次”

限流键只使用：

```go
r.RemoteAddr
```

且明确不读取代理头。

部署在 Nginx、Traefik、Caddy 或负载均衡器后，所有请求的 RemoteAddr 可能都是反向代理地址。此时任意用户消耗 60 次请求后，所有用户都会被限流。

另外：

* 注释称其为 sliding window，实际是 fixed window；
* `Retry-After` 固定写死为 60；
* cleanup goroutine 无法停止；
* visitor map 可短期无界增长。

### 建议

增加显式配置：

```text
FVF_TRUSTED_PROXIES
FVF_RATE_LIMIT
FVF_RATE_WINDOW
```

只有在请求来源属于 trusted proxy CIDR 时，才读取：

```text
CF-Connecting-IP
X-Real-IP
X-Forwarded-For
```

---

# 代码质量 Review

## 做得好的地方 ✅

### Go SSRF 防护思路正确

Go 实现不仅预先解析 DNS，还在自定义 `DialContext` 中重新解析并校验实际连接 IP，可以降低 DNS rebinding 和 TOCTOU 风险；重定向也逐跳验证。

这是项目安全设计中最成熟的部分。

### HMAC token 绑定 URL

token 包含 URL 和过期时间，使用 HMAC-SHA256 与常量时间比较，设计合理。

建议额外绑定用途：

```text
download:<url>
proxy:<url>
```

避免不同 endpoint 未来共享 token 后出现 confused deputy 问题。

### 错误信息进行了脱敏

网络拨号、DNS、TLS 等内部信息不会直接返回客户端，这一点适合公网工具。

不过这里存在一段无效代码：

```go
errors.Is(err, errors.New(""))
```

新创建的 error 永远不会匹配原 error，应删除。

### 前端避免了明显 DOM XSS

远端提供的 URL、rel、size 均通过 `textContent` 写入，而不是 `innerHTML`。

---

## 可维护性问题

### `worker.ts` 过大

Worker 后端约 670 行，同时承担：

* routing；
* SSRF；
* token；
* rate limiting；
* HTML 解析；
* Manifest；
* body limits；
* error mapping；
* response headers。

建议拆成：

```text
src/index.ts
src/routes/preview.ts
src/routes/proxy.ts
src/security/url.ts
src/security/signing.ts
src/security/rate-limit.ts
src/discovery/html.ts
src/discovery/manifest.ts
src/http/body-limit.ts
src/http/errors.ts
```

### 多处依赖字符串匹配错误

Go `PublicError()` 和 Worker `publicError()` 都通过 `strings.Contains()` 判断错误类型。这种方式容易在重构错误文案后失效。

Go 应使用 sentinel error 或 typed error：

```go
var ErrBlockedURL = errors.New("blocked URL")

type UpstreamStatusError struct {
    Status int
}
```

Worker 可以定义：

```ts
class AppError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
```

### `ProbeContentType` 存在重复分支

Go 代码中的两个分支最终执行完全相同的探测逻辑。

可以简化为：

```go
if candidates[i].Type == "" {
    candidates[i].Type = ProbeContentType(...)
}
```

---

# 格式转换与文件安全

## 任意扩展名直通逻辑过宽

转换器中存在：

```go
if target == sourceExt &&
    target != "" &&
    target != "jpg" &&
    target != "png" {
    return rawData
}
```

这意味着只要客户端传入与 URL 扩展名一致的格式，`gif`、`webp`，甚至理论上的 `html`、`xml` 都可能走直通逻辑。

虽然响应使用 attachment，风险受限，但 API 行为明显超出了文档定义。

应使用严格 allowlist：

```go
switch target {
case "png", "jpg", "svg", "ico":
default:
    return error
}
```

并通过 magic bytes 验证源内容，不应仅信任 URL 扩展名和上游 Content-Type。

## SVG/ICO 直通未验证实际内容

当 filename 以 `.svg` 结尾时，代码不检查数据是否真的是 SVG，就直接以 `image/svg+xml` 返回。

建议：

* SVG：XML token 解析，确认根节点为 `<svg>`，限制 DOCTYPE、实体和最大节点数；
* ICO：检查 ICONDIR header；
* PNG/JPEG/GIF/WebP：使用 magic bytes；
* 所有格式统一验证后再决定 allowed types。

---

# 性能与可靠性

## HTML 被解析两次

Go Preview 先调用 `DiscoverCandidates()`，之后又调用 `FindManifestHref()`，两者都完整执行 `html.Parse()`。

对于最大 5 MiB HTML，会产生两棵 DOM 树和两次遍历。

建议一次解析返回：

```go
type DiscoveryResult struct {
    Candidates   []Candidate
    ManifestHref string
}
```

## 排名算法过于简单

目前优先级先看 rel，再看 sizes：

```text
rel priority → pixel area
```

因此：

* 16×16 的 `rel=icon`
* 512×512 的 manifest icon

前者一定获胜。

更合理的评分模型可综合：

```text
SVG/vector bonus
content successfully verified
size closeness to target, such as 128×128
rel priority
same-origin preference
supported MIME
HTTP availability
```

## `sizeScore()` 应防止整数溢出

Go 直接计算：

```go
score := w * h
```

`w` 和 `h` 来自不可信 HTML。

应设置合理上限，例如每个维度最大 16384，并使用 `int64`。

---

# 测试 Review

## 当前优点

Go 测试已经覆盖：

* URL normalize；
* HTML icon discovery；
* fallback；
* manifest；
* 下载；
* PNG → JPG；
* token；
* 基础 SSRF；
* body limit；
* Web preview/download 主流程。

CI 运行：

```text
go vet ./...
go test ./...
go test -race ./...
npm run check
npm run build
```

基础门禁是合格的。

## 缺失的关键测试

建议立即增加：

| 测试                                  | 目的                  |
| ----------------------------------- | ------------------- |
| DNS rebind / redirect to private IP | 验证 SSRF 闭环          |
| 页面声明私网 favicon                      | 防止客户端侧内网请求          |
| 1000 个 icon link                    | 验证候选上限              |
| 两个同 rel/size 图标                     | 捕获文件覆盖              |
| 5 MiB、超大尺寸 PNG                      | 捕获解压炸弹              |
| HTML 冒充 `.svg`                      | 验证 magic bytes      |
| broken best candidate               | 验证自动 fallback       |
| reverse proxy headers               | 验证限流身份              |
| malformed/unquoted HTML attrs       | 保证 Go/Worker parity |
| Worker default secret missing       | 必须拒绝工作              |
| CORS scheme mismatch                | 验证严格同源              |

Worker `package.json` 只有 build/check/deploy，没有 test script。

建议采用 Cloudflare Workers Vitest integration，并直接测试 Worker 的 `fetch()` handler。

## 建议增加 fuzzing

Go 非常适合对以下函数进行 fuzz：

```text
NormalizeInputURL
DiscoverCandidates
FindManifestHref
sizeScore
safeFilename
Convert
Signer.Verify
```

这类 URL、HTML、格式解析代码非常容易从 fuzzing 获益。

---

# CI/CD 与供应链

## 做得好的地方

* Go 测试和 race detector；
* Worker TypeScript strict check；
* 多架构 Docker：amd64/arm64；
* 多平台二进制：Linux/macOS/Windows；
* Release checksums；
* Docker 非 root；
* Compose 只读 rootfs、`no-new-privileges`、tmpfs。

## 需要加强

### Container workflow 不运行测试

容器 workflow 会在 tag push 时直接 build/push，没有依赖 Release workflow 的测试结果。

这意味着一个任意 tag 可以：

* Release 测试失败；
* 但 GHCR 镜像依然成功发布。

建议把测试做成 reusable workflow，Release、Container、Deploy 都必须 `needs` 同一个验证 job。

### GitHub Actions 未 pin 到 commit SHA

当前使用：

```yaml
uses: actions/checkout@v6
uses: docker/build-push-action@v7
uses: softprops/action-gh-release@v2
```

对个人项目可以接受，但更严格的供应链策略应固定完整 commit SHA，并由 Dependabot 更新。

### 缺少安全扫描

建议加入：

```text
govulncheck ./...
npm audit --omit=dev 或 OSV Scanner
CodeQL
dependency-review-action
Trivy/Grype container scan
SBOM: syft
container provenance / cosign
```

### Go Web 的生成 JS 未纳入同步检查

Go Web 嵌入的是提交到仓库的 `app.js`，而源码是 `app.ts`。

当前 CI 只检查 Worker 目录里的 TypeScript，没有编译和比较 `internal/web/static/app.ts`。

建议 CI 执行：

```bash
npx tsc -p internal/web/static/tsconfig.json
git diff --exit-code -- internal/web/static/app.js
```

或者构建时生成，不再提交生成物。

---

# UI/UX 与无障碍

界面整体简洁，信息层级明确，移动端也有单列适配。不过还有几个问题：

* URL 输入使用 `type="text"`，应改为 `type="url"`；
* status 没有 `role="status"` 或 `aria-live="polite"`；
* `outline: none` 后没有明显的 `:focus-visible` 替代；
* 候选按钮没有 `aria-pressed`；
* 当前选中状态仅通过颜色和左边框表达；
* 加载期间只禁用按钮，没有 `aria-busy`；
* 图片加载失败没有 fallback 状态。

建议最少加入：

```html
<p id="status" role="status" aria-live="polite"></p>
<input type="url" inputmode="url" autocomplete="url">
```

```css
:focus-visible {
  outline: 2px solid currentColor;
  outline-offset: 2px;
}
```

---

# 文档和项目治理

当前 README 很适合产品主页：短、清楚、快速上手，三种使用方式一眼可见。

但之前详细的安全、API、开发说明被删掉后，没有迁移到独立文档。当前 README 只留下了一段简短部署警告。

建议建立：

```text
docs/architecture.md
docs/api.md
docs/security.md
docs/deployment-go.md
docs/deployment-worker.md
docs/threat-model.md
CONTRIBUTING.md
SECURITY.md
CHANGELOG.md
```

特别是 `SECURITY.md` 应说明漏洞报告渠道，而不是让安全问题进入公开 Issue。

---

# 推荐整改顺序

## 第一阶段：阻止高风险公网问题

1. 删除 Worker 固定默认 secret，未配置即拒绝工作。
2. Go Web 所有图标预览改走同源签名 proxy。
3. 修复 `allowedTypesFor()` 的无条件 fallback。
4. Go 增加最大像素/尺寸限制。
5. Worker Canvas 增加尺寸限制。
6. 限制候选数、Manifest icons 和探测请求数。
7. 为 Worker fetch 增加超时和 abort。

## 第二阶段：修复功能与一致性

1. 修复 `--all` 文件覆盖。
2. 最佳候选失败时自动尝试下一个。
3. 严格限制输出格式。
4. 增加文件 magic-byte 验证。
5. 建立 Go/Worker 共用 golden contract tests。
6. 修复反向代理限流。
7. 拆分 `worker.ts`。

## 第三阶段：工程化

1. Worker Vitest。
2. Go fuzz tests。
3. `govulncheck`、CodeQL、OSV、镜像扫描。
4. Actions pin SHA。
5. Release/Container/Deploy 共用验证门禁。
6. API、架构、安全和部署文档。
7. 可观测性：结构化日志、request ID、耗时、上游错误分类。

---

# 最终评价

这个项目并不是“随便拼出来的 favicon 下载器”。它已经有：

* 明确的产品形态；
* 合理的 Go 分层；
* 较强的 SSRF 意识；
* HMAC 授权；
* 体积限制；
* 容器加固；
* 多平台交付；
* 双语言 README；
* 比较干净的前端实现。

真正拖低评分的，是**两套实现的重复维护**和几个“看起来已经防护、实际上还有旁路”的安全闭环问题。

完成前述 P1 项目后，整体可提升到约 **8/10**；再补齐 Worker contract tests、发布门禁和文档后，就会成为一个相当扎实、可以放进简历重点介绍的开源工程。

[1]: https://developers.cloudflare.com/workers/platform/limits/ "https://developers.cloudflare.com/workers/platform/limits/"
[2]: https://developers.cloudflare.com/workers/platform/known-issues/ "https://developers.cloudflare.com/workers/platform/known-issues/"

