#!/usr/bin/env bash
# Writes the file tree of a Hugging Face Docker Space into an empty folder.
#
#   bash deploy/hosted/make-space.sh <empty-dir>
#
# The tree holds the tracked files of HEAD (so local-only files such as planning
# documents never reach the Space), the Dockerfile at the root where Spaces look
# for it, and the Space README with its front matter. Push the result to the
# Space's git repository yourself; this script pushes nothing.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: make-space.sh <empty-dir>" >&2
  exit 2
fi

DIR="$1"
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

mkdir -p "$DIR"
if [ -n "$(ls -A "$DIR")" ]; then
  echo "make-space: $DIR is not empty" >&2
  exit 1
fi

git -C "$ROOT" archive HEAD | tar -x -C "$DIR"
cp "$ROOT/deploy/Dockerfile" "$DIR/Dockerfile"
cp "$ROOT/deploy/hosted/space-readme.md" "$DIR/README.md"
echo "Space tree written to $DIR"
