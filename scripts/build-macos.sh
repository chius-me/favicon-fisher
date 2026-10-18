#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-0.1.0}"
if [[ ! "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
  echo "Usage: $0 MAJOR.MINOR.PATCH" >&2; exit 1
fi
BUILD="${FVF_BUILD_DIR:-$ROOT/dist/macos-build}"
APP="$ROOT/dist/Favicon Fisher.app"
export CLANG_MODULE_CACHE_PATH="${CLANG_MODULE_CACHE_PATH:-$BUILD/ModuleCache}"
export SWIFT_MODULECACHE_PATH="$CLANG_MODULE_CACHE_PATH"
mkdir -p "$BUILD" "$APP/Contents/MacOS" "$APP/Contents/Helpers" "$APP/Contents/Resources"
app_slices=()
engine_slices=()
for arch in ${FVF_ARCHS:-arm64 x86_64}; do
  case "$arch" in arm64) goarch=arm64 ;; x86_64) goarch=amd64 ;; *) echo "Unsupported architecture" >&2; exit 1 ;; esac
  swift build --package-path "$ROOT/macos" --scratch-path "$BUILD/swift-$arch" --cache-path "$BUILD/SwiftCache" --disable-sandbox \
    -c release --triple "$arch-apple-macosx14.0"
  bindir="$(swift build --package-path "$ROOT/macos" --scratch-path "$BUILD/swift-$arch" --cache-path "$BUILD/SwiftCache" --disable-sandbox \
    -c release --triple "$arch-apple-macosx14.0" --show-bin-path)"
  app_slices+=("$bindir/FaviconFisher")
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=darwin GOARCH="$goarch" go build -trimpath \
    -ldflags="-s -w -X github.com/chius-me/favicon-fisher/internal/engine.Version=$VERSION" \
    -o "$BUILD/fvf-engine-$arch" ./cmd/fvf-engine)
  engine_slices+=("$BUILD/fvf-engine-$arch")
done
lipo -create "${app_slices[@]}" -output "$APP/Contents/MacOS/FaviconFisher"
lipo -create "${engine_slices[@]}" -output "$APP/Contents/Helpers/fvf-engine"
cp "$ROOT/macos/Info.plist" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $VERSION" "$APP/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $VERSION" "$APP/Contents/Info.plist"
if [[ ! -f "$APP/Contents/Resources/AppIcon.icns" || "$ROOT/scripts/macos-icon.swift" -nt "$APP/Contents/Resources/AppIcon.icns" ]]; then
  swift "$ROOT/scripts/macos-icon.swift" "$BUILD/AppIcon.iconset"
  iconutil -c icns "$BUILD/AppIcon.iconset" -o "$APP/Contents/Resources/AppIcon.icns"
fi
cp "$ROOT/LICENSE" "$APP/Contents/Resources/LICENSE.txt"
mkdir -p "$APP/Contents/Resources/ThirdPartyLicenses"
go_root="$(go env GOROOT)"
go_license="$go_root/LICENSE"
# Homebrew moves the license one level above libexec; official Go archives do not.
if [[ ! -f "$go_license" ]]; then go_license="$(dirname "$go_root")/LICENSE"; fi
install -m 644 "$go_license" "$APP/Contents/Resources/ThirdPartyLicenses/Go-runtime.txt"
while IFS= read -r module_dir; do
  if [[ -f "$module_dir/LICENSE" ]]; then
    install -m 644 "$module_dir/LICENSE" "$APP/Contents/Resources/ThirdPartyLicenses/$(basename "$module_dir").txt"
  fi
done < <(cd "$ROOT" && go list -deps -f '{{if .Module}}{{.Module.Dir}}{{end}}' ./cmd/fvf-engine | sort -u)
identity="${MACOS_SIGNING_IDENTITY:--}"
sign_options=(--force --sign "$identity" --options runtime)
if [[ "$identity" == "-" ]]; then sign_options+=(--timestamp=none); else sign_options+=(--timestamp); fi
codesign "${sign_options[@]}" "$APP/Contents/Helpers/fvf-engine"
codesign "${sign_options[@]}" "$APP"
codesign --verify --deep --strict "$APP"
echo "Built $APP ($VERSION)"
