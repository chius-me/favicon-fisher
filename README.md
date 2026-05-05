<p align="right">
  English | <a href="./README.zh.md">简体中文</a>
</p>

<h1 align="center">favicon-fisher</h1>

<p align="center">
  A small Go CLI that discovers a website's favicon, downloads the best candidate it can find, and writes JSON-friendly metadata about the result.
</p>

<p align="center">
  <img alt="License" src="https://img.shields.io/badge/License-MIT-blue.svg">
  <img alt="Language" src="https://img.shields.io/badge/Language-Go-00ADD8.svg">
  <img alt="Releases" src="https://img.shields.io/github/v/release/chius-me/favicon-fisher?color=success">
  <img alt="CI" src="https://github.com/chius-me/favicon-fisher/actions/workflows/ci.yml/badge.svg">
</p>

## Overview

For a single URL or domain, it automatically resolves `https://`, fetches the HTML, and parses `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, and fallback `/favicon.ico` paths. It then scores the candidates, downloads the best one, and prints either a human-readable summary or structured JSON for programmatic use.

## Features

- **Automatic URL Normalization:** Understands naked domains like `github.com`.
- **Smart Discovery:** Parses all standard favicon HTML tags and handles relative vs. absolute URLs.
- **Fallback Support:** Automatically probes `/favicon.ico` if no tags are found.
- **JSON Output:** Run with `--json` to get parseable metadata.
- **Cross-Platform:** Released for Linux, macOS, and Windows via GitHub Actions.

## Quick Start

Download the latest release from the [Releases page](https://github.com/chius-me/favicon-fisher/releases) or build it from source:

```bash
go build -o favicon-fisher ./cmd/favicon-fisher
```

## Usage

Basic run, saving to a `tmp` directory:

```bash
./favicon-fisher --out tmp https://github.com
```

Output:

```text
Saved favicon
  page: https://github.com
  icon: https://github.githubassets.com/favicons/favicon.svg
  file: tmp/github.githubassets.com.svg
```

JSON mode:

```bash
./favicon-fisher --json --out tmp https://go.dev
```

Output:

```json
{
  "input_url": "https://go.dev",
  "page_url": "https://go.dev",
  "icon_url": "https://go.dev/images/favicon-gopher.svg",
  "saved_path": "tmp/go.dev.svg",
  "error": ""
}
```

## Release Builds

The repository includes a GitHub Actions release workflow.

When you push a tag starting with `v*` (e.g. `v0.1.0`), the workflow will:
1. Run all tests.
2. Cross-compile the binary for Linux, macOS, and Windows.
3. Bundle the binaries into archives.
4. Create a GitHub Release and upload the assets along with a `checksums.txt` file.

## Tests

```bash
go test ./...
```

## Project Structure

- `cmd/favicon-fisher/`: The main CLI entrypoint.
- `internal/fetcher/`: Core logic for discovery, candidate scoring, and downloading.
- `docs/plans/`: Historical implementation plans.

## Notes

- The CLI uses a generous 10-second timeout for fetching to handle slow servers.
- When an SVG is found, it is generally preferred over a PNG.