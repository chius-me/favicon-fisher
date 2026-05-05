<p align="right">
  <a href="./README.md">English</a> | 简体中文
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  一个使用 Go 编写的微型 CLI 工具，用于发现网站的 favicon，下载最佳候选图标，并输出支持 JSON 格式的结果元数据。
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-MIT-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
</p>

## 项目概览

只需提供单个 URL 或域名，工具就会自动补全 `https://`，拉取 HTML，解析 `<link rel="icon">`、`shortcut icon`、`apple-touch-icon` 以及兜底的 `/favicon.ico` 路径。随后，它会对这些候选图标进行评分，下载最佳版本，并输出人类可读的摘要信息或便于程序调用的结构化 JSON。

## 功能特性

- **URL 自动规范化：** 能够理解像 `github.com` 这样的裸域名。
- **智能发现：** 解析所有标准 favicon HTML 标签，并妥善处理相对路径与绝对 URL。
- **Fallback 支持：** 如果页面中没有提供图标标签，会自动探测 `/favicon.ico`。
- **JSON 输出：** 携带 `--json` 参数运行以获取便于解析的元数据。
- **跨平台：** 通过 GitHub Actions 提供 Linux、macOS 和 Windows 的预编译版本发布。

## 快速开始

你可以从 [Releases 页面](https://github.com/chius-me/favicon-fisher/releases) 下载最新版本，或者从源码构建：

```bash
go build -o favicon-fisher ./cmd/favicon-fisher
```

## 使用方式

基础运行，将结果保存到 `tmp` 目录：

```bash
./favicon-fisher --out tmp https://github.com
```

输出：

```text
Saved favicon
  page: https://github.com
  icon: https://github.githubassets.com/favicons/favicon.svg
  file: tmp/github.githubassets.com.svg
```

JSON 模式：

```bash
./favicon-fisher --json --out tmp https://go.dev
```

输出：

```json
{
  "input_url": "https://go.dev",
  "page_url": "https://go.dev",
  "icon_url": "https://go.dev/images/favicon-gopher.svg",
  "saved_path": "tmp/go.dev.svg",
  "error": ""
}
```

## 自动发布机制

本仓库包含 GitHub Actions 发布工作流。

当你推送以 `v*` 开头的 tag（例如 `v0.1.0`）时，工作流会自动：
1. 运行所有测试。
2. 交叉编译 Linux、macOS 和 Windows 的二进制文件。
3. 将二进制文件打包。
4. 创建 GitHub Release 并上传资产与 `checksums.txt` 校验文件。

## 测试

```bash
go test ./...
```

## 项目结构

- `cmd/favicon-fisher/`：CLI 主入口点。
- `internal/fetcher/`：包含发现、候选评分与下载的核心业务逻辑。
- `docs/plans/`：历史实现计划与记录。

## 补充说明

- 为了应对响应缓慢的服务器，CLI 在拉取时设置了 10 秒的宽容超时时间。
- 当同时发现 SVG 与 PNG 格式时，通常优先选择 SVG 版本。