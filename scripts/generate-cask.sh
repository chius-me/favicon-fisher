#!/bin/bash
set -euo pipefail
VERSION="${1:?Usage: generate-cask.sh VERSION DMG [ad-hoc|notarized]}"
DMG="${2:?DMG required}"
MODE="${3:-ad-hoc}"
case "$MODE" in ad-hoc|notarized) ;; *) echo "Unknown signing mode" >&2; exit 1 ;; esac
[[ "$VERSION" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || exit 1
[[ "$(basename "$DMG")" == "Favicon-Fisher-$VERSION-universal.dmg" ]] || { echo "Unexpected DMG filename" >&2; exit 1; }
hdiutil verify "$DMG" >&2
if [[ "$MODE" == notarized ]]; then xcrun stapler validate "$DMG" >&2; fi
SHA="$(shasum -a 256 "$DMG" | cut -d ' ' -f 1)"
cat <<EOF
cask "favicon-fisher" do
  version "$VERSION"
  sha256 "$SHA"

  url "https://github.com/chius-me/favicon-fisher/releases/download/v#{version}/Favicon-Fisher-#{version}-universal.dmg"
  name "Favicon Fisher"
  desc "Discover and save website icons from the menu bar"
  homepage "https://github.com/chius-me/favicon-fisher"

  depends_on macos: :sonoma

  app "Favicon Fisher.app"

  uninstall quit: "dev.chius.favicon-fisher"

  zap trash: [
    "~/Library/Caches/dev.chius.favicon-fisher",
    "~/Library/Preferences/dev.chius.favicon-fisher.plist",
  ]
EOF
if [[ "$MODE" == ad-hoc ]]; then
  cat <<'EOF'

  caveats <<~EOS
    This build is ad-hoc signed and is not notarized by Apple.
    If macOS blocks the first launch, review the app's source and use
    System Settings > Privacy & Security > Open Anyway to allow this app.
  EOS
EOF
fi
echo 'end'
