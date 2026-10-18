#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?Usage: package-macos.sh MAJOR.MINOR.PATCH}"
NOTARIZE="${MACOS_NOTARIZE:-false}"
case "$NOTARIZE" in
  true)
    : "${MACOS_SIGNING_IDENTITY:?Developer ID Application identity required for notarization}"
    : "${MACOS_NOTARY_PROFILE:?notarytool keychain profile required for notarization}"
    [[ "$MACOS_SIGNING_IDENTITY" == "Developer ID Application:"* ]] || { echo "Notarization requires Developer ID Application." >&2; exit 1; }
    MODE=notarized
    ;;
  false) export MACOS_SIGNING_IDENTITY=-; MODE=ad-hoc ;;
  *) echo "MACOS_NOTARIZE must be true or false" >&2; exit 1 ;;
esac
export FVF_ARCHS="arm64 x86_64"
bash "$ROOT/scripts/build-macos.sh" "$VERSION"
STAGE="$(mktemp -d "${TMPDIR:-/tmp}/fvf-dmg.XXXXXX")"
trap 'rm -rf "$STAGE"' EXIT
notary_args=(--keychain-profile "${MACOS_NOTARY_PROFILE:-}")
if [[ -n "${MACOS_NOTARY_KEYCHAIN:-}" ]]; then notary_args+=(--keychain "$MACOS_NOTARY_KEYCHAIN"); fi
notarize() {
  local file="$1" report="$2"
  xcrun notarytool submit "$file" "${notary_args[@]}" --wait --timeout 30m --output-format json > "$report"
  cat "$report"
  if [[ "$(plutil -extract status raw -o - "$report")" != "Accepted" ]]; then
    echo "Notarization was not accepted; see submission ID above for the Apple log." >&2
    return 1
  fi
}
# Staple the app before making the final disk image so the copied app also works offline.
APP="$ROOT/dist/Favicon Fisher.app"
if [[ "$NOTARIZE" == true ]]; then
  ditto -c -k --keepParent "$APP" "$STAGE/app.zip"
  notarize "$STAGE/app.zip" "$ROOT/dist/notary-app.json"
  xcrun stapler staple "$APP"
  xcrun stapler validate "$APP"
  spctl --assess --type execute --verbose=2 "$APP"
fi
codesign --verify --deep --strict "$APP"
mkdir "$STAGE/image"
ditto "$APP" "$STAGE/image/Favicon Fisher.app"
ln -s /Applications "$STAGE/image/Applications"
DMG="$ROOT/dist/Favicon-Fisher-$VERSION-universal.dmg"
hdiutil create -volname "Favicon Fisher" -srcfolder "$STAGE/image" -ov -format UDZO "$DMG"
if [[ "$NOTARIZE" == true ]]; then
  codesign --force --sign "$MACOS_SIGNING_IDENTITY" --timestamp "$DMG"
  notarize "$DMG" "$ROOT/dist/notary-dmg.json"
  xcrun stapler staple "$DMG"
  xcrun stapler validate "$DMG"
  spctl --assess --type open --context context:primary-signature --verbose=2 "$DMG"
fi
# Keep an earlier Cask intact if validation fails.
bash "$ROOT/scripts/generate-cask.sh" "$VERSION" "$DMG" "$MODE" > "$STAGE/favicon-fisher.rb"
mv "$STAGE/favicon-fisher.rb" "$ROOT/dist/favicon-fisher.rb"
shasum -a 256 "$DMG"
echo "Packaged $VERSION ($MODE)"
