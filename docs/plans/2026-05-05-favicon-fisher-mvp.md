# favicon-fisher MVP Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Build a Go CLI that discovers a website's favicon candidates, selects the best one, downloads it, and writes both the icon file and JSON metadata locally.

**Architecture:** The CLI will stay small and dependency-light. A `internal/fetcher` package will normalize input URLs, fetch HTML, parse favicon candidates from `<link>` tags plus `/favicon.ico` fallback, and download the chosen icon through an injectable HTTP client. A thin `cmd/favicon-fisher` entrypoint will parse flags, call the library, print human-readable output, and optionally emit JSON metadata.

**Tech Stack:** Go 1.26, standard library (`net/http`, `net/url`, `encoding/json`, `flag`, `os`, `path/filepath`) plus `golang.org/x/net/html` for robust HTML parsing.

---

## MVP Scope

### Included
- Accept a single URL or bare domain from CLI
- Normalize to `https://` when scheme is omitted
- Fetch homepage HTML
- Parse `<link rel="icon">`, `shortcut icon`, `apple-touch-icon`, `apple-touch-icon-precomposed`, `mask-icon`
- Resolve relative favicon URLs against the page URL
- Add `/favicon.ico` as fallback candidate
- Rank candidates and choose the best one
- Download chosen icon to an output directory
- Infer file extension from URL path or response `Content-Type`
- Write JSON metadata describing source URL, page URL, chosen icon URL, HTTP status, output path, content type, and byte size
- Print concise CLI success/failure output

### Excluded for MVP
- Batch input files
- Concurrency
- Robots/proxy configuration
- Retry/backoff policy beyond the default HTTP timeout
- Checksums/dedup store
- Crawling subpages/manifest files
- SVG rasterization or ICO frame extraction

---

## Target File Layout

- Create: `go.mod`
- Create: `cmd/favicon-fisher/main.go`
- Create: `internal/fetcher/types.go`
- Create: `internal/fetcher/fetcher.go`
- Create: `internal/fetcher/discovery.go`
- Create: `internal/fetcher/download.go`
- Create: `internal/fetcher/discovery_test.go`
- Create: `internal/fetcher/download_test.go`
- Create: `cmd/favicon-fisher/main_test.go`
- Modify later: `README.md`

---

## Task 1: Initialize Go module and repository skeleton

**Objective:** Create the module, package layout, and test command baseline before feature code exists.

**Files:**
- Create: `go.mod`
- Create: `cmd/favicon-fisher/main.go`
- Create: `internal/fetcher/types.go`

**Step 1: Initialize module**

Run:

```bash
go mod init github.com/chius-me/favicon-fisher
```

Expected: `go.mod` created.

**Step 2: Create minimal entrypoint**

Create `cmd/favicon-fisher/main.go` with a tiny `main()` that prints `favicon-fisher: not implemented` and exits 0 for now.

**Step 3: Create shared types file**

Create `internal/fetcher/types.go` with placeholder structs that will survive later tasks:

```go
package fetcher

type Candidate struct {
	URL      string
	Rel      string
	Sizes    string
	Type     string
	Priority int
}

type Result struct {
	InputURL      string `json:"input_url"`
	PageURL       string `json:"page_url"`
	IconURL       string `json:"icon_url"`
	OutputPath    string `json:"output_path"`
	ContentType   string `json:"content_type"`
	Bytes         int64  `json:"bytes"`
	StatusCode    int    `json:"status_code"`
	Filename      string `json:"filename"`
	SourceRel     string `json:"source_rel"`
	FallbackUsed  bool   `json:"fallback_used"`
}
```

**Step 4: Verify baseline builds**

Run:

```bash
go test ./...
```

Expected: pass or no-test packages only.

---

## Task 2: Write discovery tests first

**Objective:** Lock down URL normalization, HTML parsing, relative URL resolution, and fallback behavior before implementation.

**Files:**
- Create: `internal/fetcher/discovery_test.go`
- Modify: `internal/fetcher/discovery.go`

**Step 1: Write failing tests for URL normalization**

Create tests covering:
- `example.com` becomes `https://example.com`
- `http://example.com` stays unchanged
- invalid URL like `://bad` returns error

Suggested test names:

```go
func TestNormalizeInputURLAddsHTTPSWhenMissing(t *testing.T)
func TestNormalizeInputURLKeepsExistingScheme(t *testing.T)
func TestNormalizeInputURLRejectsInvalidInput(t *testing.T)
```

**Step 2: Run tests to verify RED**

Run:

```bash
go test ./internal/fetcher -run Normalize -v
```

Expected: FAIL because `NormalizeInputURL` does not exist yet.

**Step 3: Write failing tests for candidate extraction**

Use an `httptest.Server` page containing:
- `<link rel="icon" href="/favicon-32.png" sizes="32x32">`
- `<link rel="apple-touch-icon" href="https://cdn.example.com/apple.png">`
- no duplicate absolute fallback entries

Assertions:
- relative URLs are resolved against the page URL
- fallback `/favicon.ico` is appended
- icon-style rel values are ranked ahead of unknown rels

Suggested test names:

```go
func TestDiscoverCandidatesExtractsAndResolvesLinkIcons(t *testing.T)
func TestDiscoverCandidatesAddsFaviconICOFallback(t *testing.T)
func TestBestCandidatePrefersStandardIconBeforeFallback(t *testing.T)
```

**Step 4: Run tests to verify RED**

Run:

```bash
go test ./internal/fetcher -run 'Discover|BestCandidate' -v
```

Expected: FAIL because discovery functions do not exist yet.

**Step 5: Implement minimal discovery logic**

Create `internal/fetcher/discovery.go` with:
- `NormalizeInputURL(raw string) (string, error)`
- `DiscoverCandidates(pageURL string, body io.Reader) ([]Candidate, error)`
- `BestCandidate(candidates []Candidate) (Candidate, error)`
- helper for `rel` ranking

Use `golang.org/x/net/html` to parse `<link>` nodes robustly.

**Step 6: Run focused tests to verify GREEN**

Run:

```bash
go test ./internal/fetcher -run 'Normalize|Discover|BestCandidate' -v
```

Expected: PASS.

**Step 7: Refactor only if needed**
- Extract small helper functions for link attribute lookup and candidate priority
- Do not broaden scope beyond tests

---

## Task 3: Write download tests first

**Objective:** Verify HTTP download, filename/extension inference, and metadata generation before implementing downloader logic.

**Files:**
- Create: `internal/fetcher/download_test.go`
- Modify: `internal/fetcher/download.go`
- Modify: `internal/fetcher/fetcher.go`

**Step 1: Write failing test for saving a PNG favicon**

Use `httptest.Server` to return a tiny PNG byte slice with `Content-Type: image/png`.

Assertions:
- file is written under a temp output directory
- filename includes host and `.png`
- metadata reports byte size and content type

Suggested test name:

```go
func TestDownloadIconSavesPNGAndReturnsMetadata(t *testing.T)
```

**Step 2: Run test to verify RED**

Run:

```bash
go test ./internal/fetcher -run DownloadIconSavesPNG -v
```

Expected: FAIL because `DownloadIcon` does not exist yet.

**Step 3: Write failing test for extension inference fallback**

Serve bytes with URL path lacking extension and content type `image/x-icon`.

Assertions:
- output file ends in `.ico`
- metadata captures chosen filename

Suggested test name:

```go
func TestDownloadIconInfersICOExtensionFromContentType(t *testing.T)
```

**Step 4: Write failing test for non-success responses**

Serve `404` and assert a typed error or message that includes status code.

Suggested test name:

```go
func TestDownloadIconReturnsErrorOnNonSuccessStatus(t *testing.T)
```

**Step 5: Run tests to verify RED**

Run:

```bash
go test ./internal/fetcher -run DownloadIcon -v
```

Expected: FAIL.

**Step 6: Implement minimal download logic**

Add `internal/fetcher/download.go` with:
- `DownloadIcon(ctx context.Context, client *http.Client, iconURL string, outputDir string) (Result, error)`
- helpers to infer extension from URL path/content type
- safe filename generation using host plus extension

Add `internal/fetcher/fetcher.go` with a thin orchestrator:

```go
type Fetcher struct {
	Client *http.Client
}

func New(client *http.Client) *Fetcher
func (f *Fetcher) Fetch(ctx context.Context, rawURL string, outputDir string) (Result, error)
```

`Fetch` should:
- normalize input URL
- GET homepage
- discover candidates
- choose best candidate
- download chosen icon
- enrich metadata with original input, page URL, source rel, fallback flag

**Step 7: Run tests to verify GREEN**

Run:

```bash
go test ./internal/fetcher -v
```

Expected: PASS.

---

## Task 4: Write CLI tests first

**Objective:** Confirm flags, JSON output, and filesystem behavior at the command boundary.

**Files:**
- Create: `cmd/favicon-fisher/main_test.go`
- Modify: `cmd/favicon-fisher/main.go`

**Step 1: Refactor main logic behind a testable runner**

Plan for this shape:

```go
func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int
```

`main()` should only call `os.Exit(run(...))`.

**Step 2: Write failing test for missing argument**

Assert:
- exit code is non-zero
- stderr contains usage guidance

Suggested test name:

```go
func TestRunReturnsErrorWhenURLMissing(t *testing.T)
```

**Step 3: Write failing test for successful JSON output**

Inject a fake fetch function instead of real network calls.

Assertions:
- `--json` prints valid JSON
- output path and icon URL appear in payload
- exit code is 0

Suggested test name:

```go
func TestRunPrintsJSONOnSuccess(t *testing.T)
```

**Step 4: Write failing test for human-readable success output**

Assertions:
- stdout includes chosen icon URL and saved path
- exit code is 0

Suggested test name:

```go
func TestRunPrintsHumanReadableSummary(t *testing.T)
```

**Step 5: Run tests to verify RED**

Run:

```bash
go test ./cmd/favicon-fisher -v
```

Expected: FAIL.

**Step 6: Implement minimal CLI**

Support flags:
- `--out DIR` default `./out`
- `--json` to print JSON metadata only
- positional single URL

Human-readable success format:

```text
Saved favicon
  page: <page-url>
  icon: <icon-url>
  file: <output-path>
```

**Step 7: Run tests to verify GREEN**

Run:

```bash
go test ./cmd/favicon-fisher -v
```

Expected: PASS.

---

## Task 5: End-to-end verification and docs

**Objective:** Prove the CLI works outside unit tests and document the MVP.

**Files:**
- Modify: `README.md`

**Step 1: Run full test suite**

Run:

```bash
go test ./...
```

Expected: all packages PASS.

**Step 2: Build the CLI**

Run:

```bash
go build ./cmd/favicon-fisher
```

Expected: binary builds without errors.

**Step 3: Manual smoke test against a known site**

Run:

```bash
mkdir -p tmp/manual
./favicon-fisher --out tmp/manual https://example.com
```

Expected:
- one icon file appears under `tmp/manual`
- CLI prints saved path and icon URL

**Step 4: Verify JSON mode**

Run:

```bash
./favicon-fisher --json --out tmp/manual https://example.com
```

Expected: valid JSON object on stdout.

**Step 5: Update README**

Document:
- what the tool does
- install/build instructions
- CLI usage examples
- MVP limitations

---

## Notes for Implementation

- Prefer `http.Client{Timeout: 15 * time.Second}`
- Reject homepage HTTP errors before parsing HTML
- Treat empty or whitespace-only `href` as absent
- Resolve `//cdn.example.com/icon.png` URLs using the page scheme
- When multiple standard icons exist, prefer larger declared `sizes` over smaller if both are otherwise equal
- Keep package APIs small; no premature abstractions
- All production code must come after a failing test

## Verification Checklist

- [ ] `go test ./...` passes
- [ ] `go build ./cmd/favicon-fisher` passes
- [ ] CLI succeeds for `https://example.com`
- [ ] JSON output is machine-readable
- [ ] README documents current MVP behavior, not future ideas
