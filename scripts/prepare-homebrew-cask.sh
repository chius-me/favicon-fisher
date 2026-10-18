#!/bin/bash
# Download only published stable assets and generate a Cask from the verified DMG.
# Does not write to GitHub or modify an installed Homebrew tap.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TAG="${1:?Usage: prepare-homebrew-cask.sh vMAJOR.MINOR.PATCH OUTPUT_DIRECTORY [ad-hoc|notarized]}"
OUTPUT="${2:?Output directory required}"
MODE="${3:-ad-hoc}"
case "$MODE" in ad-hoc|notarized) ;; *) echo "Unknown signing mode" >&2; exit 1 ;; esac
[[ "$TAG" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { echo "A stable vMAJOR.MINOR.PATCH tag is required" >&2; exit 1; }
REPO="chius-me/favicon-fisher"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/fvf-cask.XXXXXX")"
trap 'rm -rf "$WORK"' EXIT
gh release view "$TAG" --repo "$REPO" --json tagName,isDraft,isPrerelease > "$WORK/release.json"
python3 - "$WORK/release.json" "$TAG" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    release = json.load(f)
if release["tagName"] != sys.argv[2] or release["isDraft"] or release["isPrerelease"]:
    sys.exit("Only a published stable release can be added to the tap")
PY
DMG="Favicon-Fisher-${TAG#v}-universal.dmg"
gh release download "$TAG" --repo "$REPO" --pattern "$DMG" --pattern checksums.txt --dir "$WORK"
# Validate only the expected asset, never paths supplied by the checksum manifest.
python3 - "$WORK" "$DMG" <<'PY'
import hashlib, pathlib, re, sys
root, name = pathlib.Path(sys.argv[1]), sys.argv[2]
matches = []
for line in (root / "checksums.txt").read_text().splitlines():
    match = re.fullmatch(r"([0-9a-fA-F]{64}) [ *](.+)", line)
    if match and match[2] == name:
        matches.append(match[1].lower())
with (root / name).open("rb") as f:
    hasher = hashlib.sha256()
    for chunk in iter(lambda: f.read(1024 * 1024), b""):
        hasher.update(chunk)
    digest = hasher.hexdigest()
if matches != [digest]:
    sys.exit("Missing, duplicate or mismatched DMG checksum")
PY
bash "$ROOT/scripts/generate-cask.sh" "${TAG#v}" "$WORK/$DMG" "$MODE" > "$WORK/favicon-fisher.rb"
mkdir -p "$OUTPUT"
cp "$WORK/favicon-fisher.rb" "$OUTPUT/favicon-fisher.rb"
echo "Prepared $OUTPUT/favicon-fisher.rb from verified $TAG"
