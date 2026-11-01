#!/usr/bin/env bash
# Fetches the Bifrost source needed by deploy/Dockerfile into .cache/bifrost-src.
# Only the transports/ and ui/ folders are checked out; the Go modules for core,
# framework and plugins come from the Go module proxy.
set -euo pipefail

TAG="${BIFROST_TAG:-transports/v2.4.0}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DEST="$ROOT/.cache/bifrost-src"
# Written as the last step, so an interrupted run is fetched again.
MARKER="$DEST/.fetch-complete"

if [ -f "$MARKER" ] && [ "$(cat "$MARKER")" = "$TAG" ]; then
  echo "Bifrost $TAG is already in $DEST"
  exit 0
fi

rm -rf "$DEST"
mkdir -p "$ROOT/.cache"
git -c core.autocrlf=false clone --depth 1 --filter=blob:none --sparse --branch "$TAG" \
  https://github.com/maximhq/bifrost.git "$DEST"
# Keep LF line endings: the files are used inside Linux containers.
git -C "$DEST" config core.autocrlf false
git -C "$DEST" config core.eol lf
git -C "$DEST" sparse-checkout set transports ui
# Rewrite files checked out before the settings above took effect.
git -C "$DEST" rm -rq --cached .
git -C "$DEST" reset -q --hard
echo "$TAG" > "$MARKER"
echo "Bifrost $TAG checked out in $DEST"
