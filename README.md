# favicon-fisher

A small Go CLI that discovers a website's favicon, downloads the best candidate it can find, and writes JSON-friendly metadata about the result.

## What it does

For a single URL or bare domain, `favicon-fisher`:

- normalizes the input to a usable URL
- fetches the homepage HTML
- parses favicon candidates from common `<link rel="...icon...">` tags
- adds `/favicon.ico` as a fallback candidate
- chooses the best candidate by rel priority and declared size
- downloads the chosen icon into a local output directory
- prints either a human-readable summary or JSON metadata

## Requirements

- Go 1.26+

## Build

```bash
go build -o favicon-fisher ./cmd/favicon-fisher
```

## Release builds

The repository includes a GitHub Actions release workflow at `.github/workflows/release.yml`.

When you push a tag that starts with `v`, GitHub Actions will:

- run `go test ./...`
- cross-compile release binaries for:
  - `linux/amd64`
  - `linux/arm64`
  - `darwin/amd64`
  - `darwin/arm64`
  - `windows/amd64`
  - `windows/arm64`
- package each binary with `README.md` and `LICENSE`
- generate `checksums.txt`
- publish all archives to the matching GitHub Release

### Create a release

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow will create a GitHub Release for `v0.1.0` automatically.

## Usage

```bash
./favicon-fisher [--out DIR] [--json] <url>
```

Examples:

```bash
./favicon-fisher https://github.com
./favicon-fisher --out tmp/icons https://go.dev
./favicon-fisher --json --out tmp/icons github.com
```

## Flags

- `--out DIR`
  - output directory for the downloaded favicon
  - default: `./out`
- `--json`
  - print the result as JSON instead of a human-readable summary

## Example human output

```text
Saved favicon
  page: https://github.com/
  icon: https://github.githubassets.com/favicons/favicon.svg
  file: tmp/icons/github.githubassets.com.svg
```

## Example JSON output

```json
{
  "input_url": "github.com",
  "page_url": "https://github.com/",
  "icon_url": "https://github.githubassets.com/favicons/favicon.svg",
  "output_path": "tmp/icons/github.githubassets.com.svg",
  "content_type": "image/svg+xml",
  "bytes": 6518,
  "status_code": 200,
  "filename": "github.githubassets.com.svg",
  "source_rel": "icon",
  "fallback_used": false
}
```

## Current MVP limitations

This first version intentionally does **not** yet support:

- batch fetching multiple sites
- concurrent downloads
- manifest crawling
- retry/backoff tuning
- deduplication or hashing
- richer favicon scoring using dimensions beyond declared `sizes`
- extracting multiple icons from `.ico` containers

## Notes

- Some sites simply do not expose a working favicon endpoint. For example, `https://example.com` currently returns `404` for `/favicon.ico`, so the CLI correctly reports an error there.
- Bare domains like `github.com` are automatically normalized to `https://github.com`.
