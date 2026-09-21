#!/bin/sh
set -eu
version=${1:-v0.0.0-prototype}
case "$version" in ''|*[!A-Za-z0-9._-]*) echo 'Invalid version' >&2;exit 1;; esac
mkdir -p dist
for platform in darwin linux windows;do
 for arch in amd64 arm64;do
  suffix=;[ "$platform" != windows ] || suffix=.exe
  CGO_ENABLED=0 GOOS=$platform GOARCH=$arch go build -trimpath -ldflags "-X main.version=$version" -o "dist/tofa_${version}_${platform}_${arch}${suffix}" ./cmd/tofa
 done
done
(cd dist; if command -v sha256sum >/dev/null 2>&1;then sha256sum tofa_* > SHA256SUMS;else shasum -a 256 tofa_* > SHA256SUMS;fi)
printf '%s\n' 'Built six artifacts. Cross-compilation does not establish native execution or model compatibility.'
