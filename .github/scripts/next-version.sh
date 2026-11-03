#!/usr/bin/env bash
# Prints the version after the latest vX.Y.Z tag, bumped by $1 (major, minor or patch).
# Needs the tags fetched (actions/checkout with fetch-depth: 0).
set -euo pipefail

# Only plain vX.Y.Z tags count; something like v1.2.0-rc1 would break the arithmetic below.
latest=$(git tag --list 'v*' --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n1 || true)
IFS=. read -r major minor patch <<< "${latest:-v0.0.0}"
major=${major#v}

case "${1:-}" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "usage: $0 major|minor|patch" >&2; exit 1 ;;
esac

echo "$major.$minor.$patch"
