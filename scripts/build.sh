#!/bin/sh
set -eu
[ "$#" -eq 1 ] || { echo 'Usage: sh scripts/build.sh VERSION (for example v0.1.0-rc.1)' >&2; exit 1; }
version=$1
case "$version" in ''|latest|.|..|*[!A-Za-z0-9._-]*) echo 'Invalid explicit version' >&2;exit 1;; esac
# Publish a complete fresh directory only after every build succeeds.
[ ! -L dist ] || { echo 'Refusing symlink distribution directory' >&2; exit 1; }
work=$(mktemp -d ./.tofa-dist.XXXXXX)
trap 'rm -rf "$work"' EXIT
trap 'exit 1' HUP INT TERM
cp LICENSE README.md THIRD_PARTY_NOTICES.txt "$work/"
cp scripts/install.sh scripts/install.ps1 scripts/uninstall.sh scripts/uninstall.ps1 "$work/"
cp "$(go env GOROOT)/LICENSE" "$work/LICENSE-GO.txt"
cp "$(go env GOROOT)/PATENTS" "$work/PATENTS-GO.txt"
cp internal/tofa/assets/codex-LICENSE "$work/LICENSE-CODEX.txt"
cp internal/tofa/assets/codex-NOTICE "$work/NOTICE-CODEX.txt"
for platform in darwin linux windows;do
 for arch in amd64 arm64;do
  suffix=;[ "$platform" != windows ] || suffix=.exe
  CGO_ENABLED=0 GOOS=$platform GOARCH=$arch go build -trimpath -ldflags "-X main.version=$version" -o "$work/tofa_${version}_${platform}_${arch}${suffix}" ./cmd/tofa
 done
done
(cd "$work"; if command -v sha256sum >/dev/null 2>&1;then sha256sum * > SHA256SUMS;else shasum -a 256 * > SHA256SUMS;fi)
rm -rf dist
mv "$work" dist
printf '%s\n' 'Built six artifacts. Cross-compilation does not establish native execution or model compatibility.'
