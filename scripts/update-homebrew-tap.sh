#!/bin/bash
# Run against a dedicated, clean checkout of the tap's main branch.
set -euo pipefail
TAG="${1:?Usage: update-homebrew-tap.sh TAG VERIFIED_CASK TAP_CHECKOUT [local|pr]}"
CASK="${2:?Verified generated Cask required}"
TAP="${3:?Dedicated tap checkout required}"
MODE="${4:-local}"
case "$MODE" in local|pr) ;; *) echo "Unknown update mode" >&2; exit 1 ;; esac
[[ "$TAG" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || exit 1
VERSION="${TAG#v}"
BRANCH="codex/favicon-fisher-$VERSION"
REPO="chius-me/homebrew-tap"
CASK="$(cd "$(dirname "$CASK")" && pwd)/$(basename "$CASK")"
cd "$TAP"
[[ "$(git branch --show-current)" == main && -z "$(git status --porcelain)" ]] || { echo "Use a clean checkout of tap main" >&2; exit 1; }
python3 - "$VERSION" "$CASK" <<'PY'
import pathlib, re, sys
def version(path):
    match = re.search(r'^  version "(\d+\.\d+\.\d+)"$', path.read_text(), re.M)
    if not match:
        sys.exit(f"Unrecognized Cask version: {path}")
    return tuple(map(int, match[1].split(".")))
requested = tuple(map(int, sys.argv[1].split(".")))
generated = pathlib.Path(sys.argv[2])
current = pathlib.Path("Casks/favicon-fisher.rb")
if version(generated) != requested:
    sys.exit("Generated Cask version does not match the requested tag")
if current.exists():
    existing = version(current)
    if existing > requested:
        sys.exit("Refusing to downgrade the tap")
    if existing == requested and current.read_bytes() != generated.read_bytes():
        sys.exit("Refusing to replace an already published version with different contents")
PY
if [[ -f Casks/favicon-fisher.rb ]] && cmp -s "$CASK" Casks/favicon-fisher.rb; then
  echo "Tap already contains $TAG"
  exit 0
fi
if [[ "$MODE" == local ]]; then
  mkdir -p Casks
  cp "$CASK" Casks/favicon-fisher.rb
  echo "Updated $PWD/Casks/favicon-fisher.rb; review, commit and push with your existing Git credentials."
  exit 0
fi
if git show-ref --verify --quiet "refs/remotes/origin/$BRANCH"; then
  git switch -c "$BRANCH" "origin/$BRANCH"
else
  git switch -c "$BRANCH"
fi
mkdir -p Casks
cp "$CASK" Casks/favicon-fisher.rb
git add Casks/favicon-fisher.rb
if ! git diff --cached --quiet; then
  git -c user.name='github-actions[bot]' -c user.email='41898282+github-actions[bot]@users.noreply.github.com' \
    commit -m "Add Favicon Fisher $VERSION cask"
fi
git push origin "$BRANCH"
PR="$(gh pr list --repo "$REPO" --head "$BRANCH" --base main --state open --json url --jq '.[0].url // empty')"
if [[ -z "$PR" ]]; then
  BODY="$(mktemp "${TMPDIR:-/tmp}/fvf-tap-pr.XXXXXX")"
  trap 'rm -f "$BODY"' EXIT
  cat > "$BODY" <<EOF
Add Favicon Fisher $VERSION, a native macOS menu bar app for discovering and saving website icons.

The Universal DMG is published at https://github.com/chius-me/favicon-fisher/releases/tag/$TAG.
Its SHA-256 matches the release manifest. Homebrew style, online audit, installation, and installed-app signature/architecture checks passed on the macOS runner. Free builds disclose their ad-hoc signing and first-launch instructions in the Cask caveats; notarized builds additionally validate Apple's ticket.

Before merging, verify menu-bar launch and an icon export on a Mac. To reproduce the installation checks:

\`\`\`bash
brew audit --cask --strict --online chius-me/tap/favicon-fisher
brew install --cask chius-me/tap/favicon-fisher
\`\`\`

Run these against this PR's branch in the local tap checkout. Confirm main does not already contain a newer version before merging.
EOF
  PR="$(gh pr create --repo "$REPO" --base main --head "$BRANCH" --title "favicon-fisher $VERSION" --body-file "$BODY")"
fi
echo "$PR"
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then printf 'Homebrew Cask PR: %s\n' "$PR" >> "$GITHUB_STEP_SUMMARY"; fi
