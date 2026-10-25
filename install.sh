#!/bin/sh
# Install a twitter-use release binary into ~/.local/bin.
#
#   curl -fsSL https://raw.githubusercontent.com/yoyooyooo/twitter-use/main/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/yoyooyooo/twitter-use/main/install.sh | sh -s -- v0.1.2
#
# Resolves the latest release, checks SHA256SUMS, then writes twitter.
# A GH_TOKEN or GITHUB_TOKEN uses the releases API. Otherwise the public release redirect supplies the tag.
# Override with TWITTER_USE_VERSION, TWITTER_INSTALL_DIR, or TWITTER_USE_REPO.
set -eu

REPO="${TWITTER_USE_REPO:-yoyooyooo/twitter-use}"
VERSION="${TWITTER_USE_VERSION:-latest}"
DEST="${TWITTER_INSTALL_DIR:-$HOME/.local/bin}"

while [ $# -gt 0 ]; do
  case "$1" in
    -v|--version)
      VERSION="${2:?missing version}"
      shift 2
      ;;
    --prefix)
      DEST="${2:?missing prefix}"
      shift 2
      ;;
    -h|--help)
      echo "usage: install.sh [vX.Y.Z] [--version vX.Y.Z] [--prefix DIR]"
      exit 0
      ;;
    v*)
      VERSION="$1"
      shift
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

os=$(uname -s)
arch=$(uname -m)
case "$os:$arch" in
  Darwin:arm64) asset="twitter-darwin-arm64" ;;
  Darwin:x86_64) asset="twitter-darwin-amd64" ;;
  Linux:x86_64) asset="twitter-linux-amd64" ;;
  Linux:aarch64|Linux:arm64) asset="twitter-linux-arm64" ;;
  *)
    echo "unsupported $os/$arch" >&2
    exit 1
    ;;
esac

fetch() {
  curl -fsSL --retry 5 --retry-delay 2 --max-time 180 \
    -H "User-Agent: twitter-use-install" \
    "$1" -o "$2"
}

resolve_latest_tag() {
  token="${GH_TOKEN:-${GITHUB_TOKEN:-}}"
  if [ -n "$token" ]; then
    if curl -fsSL --retry 3 --retry-delay 2 --max-time 30 \
      -H "User-Agent: twitter-use-install" \
      -H "Accept: application/vnd.github+json" \
      -H "Authorization: Bearer ${token}" \
      "https://api.github.com/repos/${REPO}/releases/latest" -o "$tmp/release.json"; then
      tag=$(sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p' "$tmp/release.json" | head -n 1)
      if [ -n "$tag" ]; then
        return 0
      fi
    fi
  fi
  loc=$(curl -fsS --retry 3 --retry-delay 2 --max-time 30 \
    -H "User-Agent: twitter-use-install" \
    -o /dev/null -w '%{redirect_url}' \
    "https://github.com/${REPO}/releases/latest/download/SHA256SUMS" || true)
  tag=$(printf '%s\n' "$loc" | sed -n 's|.*/releases/download/\(v[^/]*\)/.*|\1|p')
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

tag=""
if [ "$VERSION" = "latest" ]; then
  resolve_latest_tag
  if [ -z "$tag" ]; then
    echo "could not resolve the latest release" >&2
    exit 1
  fi
  base="https://github.com/${REPO}/releases/download/${tag}"
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
  base="https://github.com/${REPO}/releases/download/${tag}"
fi

fetch "${base}/SHA256SUMS" "$tmp/SHA256SUMS"
fetch "${base}/${asset}" "$tmp/${asset}"

expected=$(awk -v name="$asset" '$2 == name { print $1 }' "$tmp/SHA256SUMS")
if [ -z "$expected" ]; then
  echo "SHA256SUMS has no entry for $asset" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/${asset}" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
  got=$(shasum -a 256 "$tmp/${asset}" | awk '{ print $1 }')
else
  echo "need sha256sum or shasum" >&2
  exit 1
fi
if [ "$got" != "$expected" ]; then
  echo "checksum mismatch for $asset" >&2
  exit 1
fi

mkdir -p "$DEST"
if command -v install >/dev/null 2>&1; then
  install -m 755 "$tmp/${asset}" "$DEST/twitter"
else
  cp "$tmp/${asset}" "$DEST/twitter"
  chmod 755 "$DEST/twitter"
fi

echo "installed ${tag} -> $DEST/twitter"
case ":$PATH:" in
  *":$DEST:"*) ;;
  *) echo "add $DEST to PATH" ;;
esac
