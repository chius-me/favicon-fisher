# macOS menu bar app

Favicon Fisher is a native SwiftUI menu bar app with a bundled Go engine. It accesses websites directly; it does not require the CLI, a local web server, or the Cloudflare Worker.

## Build and run locally

Requires macOS 14+, Xcode (Swift 5.9+), and the Go version in `go.mod`.

```bash
bash scripts/build-macos.sh 1.6.0
open "dist/Favicon Fisher.app"
```

The default build contains **both arm64 and x86_64** slices, including its helper. It is ad-hoc signed by default. This free build can be distributed through the custom Homebrew tap, with a first-launch Gatekeeper prompt. For a faster Apple Silicon development build, set `FVF_ARCHS=arm64`. Open `macos/Package.swift` in Xcode to edit the Swift package. A package plus reproducible app-bundle script replaces a hand-maintained `.xcodeproj`.

The application lives in the menu bar, with no Dock icon. The menu bar and panel use the same minimal browser-and-favicon mark as the app icon; the menu-bar image is a monochrome template that follows macOS appearance, including while fetching. Enter a domain/URL or click **粘贴并抓取**, then select a candidate. Save supports original files, PNG and JPEG for raster sources. Dragging exports the original file. PNG copies preserve transparency; JPEG conversion uses a white background. GIF conversion uses the first frame. SVG is file-only; ICO preview uses ImageIO, with original ICO export.

History stores the most recent 20 URLs locally, without userinfo, query or fragment. Disabling history clears it. Login at startup is opt-in. Cache cleanup runs at launch, removing entries older than seven days and oldest entries above 100 MiB; current-session files are kept until a future launch so file drags remain valid. The cache is not a hard per-session storage quota.

If a trusted local TUN proxy maps public DNS names to `198.18.0.0/15`, enable **兼容 TUN/Fake-IP 代理** in Settings. It is off by default. This permits only that benchmark range for the desktop helper; loopback, RFC1918 and other private ranges remain blocked. Domain-to-destination resolution in this mode relies on the local proxy. It does not enable private-network access for the Web or Worker service.

## Tests

```bash
go test ./...
go vet ./...
go test -race ./...
go build -o /tmp/fvf-engine ./cmd/fvf-engine
FVF_TEST_ENGINE=/tmp/fvf-engine swift test --package-path macos
```

Swift tests cover protocol integration with the real helper, cache path validation and history privacy. Go tests cover discovery without speculative image requests, image validation, real WebP decoding, bounded thumbnails, export, private destinations, traversal and cancellation. Worker tests additionally cover IPv6 private and mapped addresses.

`verify.yml` builds a Universal app and runs the Swift/helper tests plus offline release regression tests on macOS. The release tests stub GitHub and Apple services while checking real SHA-256 digests and local Git branches: unpublished releases, malformed versions, missing/duplicate/mismatched checksums, invalid tickets, downgrades, changed published versions, dirty checkouts, and idempotent retries. Manual release QA still needs both hardware architectures, the oldest supported macOS, login-item behavior, multi-monitor use, Finder drag/save, sleep/wake and clean-machine Gatekeeper validation.

### Local verification (2026-10-18)

- Universal 2 app and helper built at version 1.6.0; strict nested ad-hoc signature verification passed. Bundle size is approximately 18 MiB.
- Go unit tests, race detector and vet passed. Swift/helper tests: 5 passed, including process cancellation. Worker checks/build/golden and 29 tests passed.
- Live GitHub discovery returned 15 candidates. With the explicit Fake-IP option required by this machine's TUN DNS, both original SVG export and a 512×512 PNG source exported as PNG/JPEG succeeded.
- Native UI automation timed out before returning window state. Menu-bar opening, actual clipboard/save/drag actions, login-item registration and visual layout have not been manually accepted. These are implemented but still require UI QA.
- Free distribution uses ad-hoc signing. Apple notarization is optional and has not been performed.

## Engine protocol v1

`Contents/Helpers/fvf-engine` accepts **one newline-terminated JSON request** on stdin and writes one JSON response on stdout. Keep stdin open until receiving the response: EOF cancels a running request. stderr is reserved for diagnostics. The native client drains both pipes concurrently, bounds output and terminates cancelled or expired processes. The engine has its own 35-second watchdog.

Request fields:

| Field | Meaning |
| --- | --- |
| `protocol_version` | Required, currently `1` |
| `id` | Required correlation ID, max 128 characters |
| `op` | `capabilities`, `discover`, `fetch`, `export` |
| `url` | Website for discover; candidate URL for fetch |
| `cache_dir` | Existing absolute private cache directory for fetch/export |
| `source` | Basename of a cached file for export |
| `format` | `original`, `png`, `jpg` for export, subject to source capability |
| `timeout_ms` | Per-operation budget, up to 30000; defaults to 15000 |
| `allow_fake_ip` | Desktop-only opt-in for a trusted TUN mapping in 198.18.0.0/15; defaults to false |

The result envelope contains `protocol_version`, `id`, `ok`, and either `result` or `error: {code, message}`. Discovery returns ranked `candidates`; files return `path`, optional `preview_path`, actual format/dimensions, byte count and `allowed_formats`. Paths are relative to the supplied cache directory. Candidate IDs are UI correlation identifiers, not authorization tokens. Every network fetch revalidates its destination.

## Public release

The default release is free: a Universal app with ad-hoc signing, distributed as a DMG through GitHub Releases and [chius-me/homebrew-tap](https://github.com/chius-me/homebrew-tap). No Apple membership, Apple credentials, or extra GitHub token is required for local tap updates.

The app is **not notarized by Apple**. Homebrew retains its normal quarantine behavior. If macOS blocks the first launch, review the source/download provenance and use **System Settings → Privacy & Security → Open Anyway** for this app. Neither the Cask nor scripts disable Gatekeeper or remove quarantine. See [Apple's instructions](https://support.apple.com/guide/mac-help/open-a-mac-app-from-an-unidentified-developer-mh40616/mac).

### Build the free DMG

```bash
bash scripts/package-macos.sh 1.6.0
```

This builds both architectures, ad-hoc signs the app/helper, verifies their signatures, creates and verifies the DMG, and writes `dist/favicon-fisher.rb` with the actual SHA-256 and first-launch caveats. `MACOS_NOTARIZE=false` is the default and explicitly selects ad-hoc signing even if another signing identity is present in the environment.

### Release and update the local tap

A new stable `vMAJOR.MINOR.PATCH` tag runs the shared verification gate and builds the macOS DMG alongside CLI binaries. The release job uploads the assets and `checksums.txt`. No secret/enable variable is needed for the free macOS release. Never replace an already published version; create a new version for changes.

Once the release is published, use the existing local tap clone:

```bash
bash scripts/prepare-homebrew-cask.sh v1.6.0 /tmp/fvf-cask
bash scripts/update-homebrew-tap.sh v1.6.0 /tmp/fvf-cask/favicon-fisher.rb "$HOME/repo/github/homebrew-tap"
git -C "$HOME/repo/github/homebrew-tap" diff -- Casks/favicon-fisher.rb
git -C "$HOME/repo/github/homebrew-tap" add Casks/favicon-fisher.rb
git -C "$HOME/repo/github/homebrew-tap" commit -m 'Add Favicon Fisher 1.6.0 cask'
git -C "$HOME/repo/github/homebrew-tap" push origin main
```

Preparation downloads only a published stable release, verifies the DMG checksum and disk-image integrity, and regenerates the Cask. The local update mode requires a clean `main` checkout, rejects downgrades or replacements of the same published version, and changes only `Casks/favicon-fisher.rb`. It does not commit, push, create a PR, or require `HOMEBREW_TAP_TOKEN`; use your existing Git/SSH credentials for the subsequent commands.

### Install and validate

After the Cask is pushed:

```bash
brew tap chius-me/tap
brew update
brew style --cask chius-me/tap/favicon-fisher
brew audit --cask --strict --online chius-me/tap/favicon-fisher
brew install --cask chius-me/tap/favicon-fisher
open "/Applications/Favicon Fisher.app"
```

The app appears in the menu bar, without a Dock icon. If the first launch is blocked, follow the per-app Privacy & Security instructions above. This custom tap's free build does not claim to satisfy the official `homebrew/cask` repository's signing/notarization policy.

```bash
brew upgrade --cask chius-me/tap/favicon-fisher
brew uninstall --cask favicon-fisher
# Optional: also remove app cache/preferences/history.
brew uninstall --cask --zap favicon-fisher
```

Ordinary uninstall preserves history; zap removes only the application's cache/preferences. User-exported images are never removed. There is no in-app updater.

### Optional: paid signing and notarization

Only if you later want Developer ID signing and notarization, follow [中文凭据指南](macos-release-setup.zh.md). Configure the six Apple-related secrets described there, then set repository variable `MACOS_NOTARIZE=true`. The same release job will notarize/staple both the app and DMG and require Gatekeeper acceptance; a failure blocks publication instead of silently falling back to an ad-hoc release.

Local equivalent:

```bash
MACOS_NOTARIZE=true \
MACOS_SIGNING_IDENTITY='Developer ID Application: YOUR NAME (TEAMID)' \
MACOS_NOTARY_PROFILE=fvf-notary \
bash scripts/package-macos.sh 1.6.0
```

For a notarized asset, pass `notarized` as the third argument to `prepare-homebrew-cask.sh`. This requires a valid stapled ticket and omits the free-build caveat. Notary submission JSON is saved in `dist/notary-*.json`; failed CI submissions retain these diagnostics for seven days. Inspect the submission ID with `xcrun notarytool info` / `log` if Apple times out.

### Optional: automatic cross-repository tap PR

Local Git updates are the default. For CI to open tap PRs instead, configure `HOMEBREW_TAP_TOKEN` with Contents and Pull requests write access limited to `chius-me/homebrew-tap`, and set `HOMEBREW_TAP_AUTOMATION=true`. The optional workflow runs style, online audit, temporary installation and signature/architecture checks before opening/reusing a version PR. It checks notarization only when explicitly enabled and never auto-merges. Retry an already-published release with:

```bash
gh workflow run update-homebrew.yml --repo chius-me/favicon-fisher -f tag=v1.6.0
# Add -f notarized=true only for a notarized release.
```

Manual workflow dispatch requires the workflow on the default branch. Tap updates are serialized. A failed tap update does not remove the already published release. See [Homebrew's Cask Cookbook](https://docs.brew.sh/Cask-Cookbook) and [GitHub reusable workflows](https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows).
