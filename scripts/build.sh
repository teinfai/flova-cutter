#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export GOCACHE="$PWD/.cache/go-build"
export GOTOOLCHAIN=local GOPROXY=off CGO_ENABLED=0

unformatted=$(gofmt -l .)
if [[ -n "$unformatted" ]]; then
  printf '%s\n' "$unformatted" >&2
  exit 1
fi
go vet ./...

mkdir -p .cache dist
stage=$(mktemp -d "$PWD/.cache/package.XXXXXXXX")
trap 'rm -rf "$stage"' EXIT
app="dist/Flova Cutter.app"
mkdir -p "$app/Contents/MacOS"

for arch in arm64 amd64; do
  GOOS=darwin GOARCH="$arch" go build -trimpath -ldflags "-s -w" -o "$stage/$arch" .
done
lipo -create "$stage/arm64" "$stage/amd64" -output "$app/Contents/MacOS/flova-cutter"
cp packaging/Info.plist "$app/Contents/Info.plist"
codesign --force -s - "$app"
codesign --verify --strict "$app"
ditto -c -k --keepParent "$app" "$stage/FlovaCutter-mac.zip"
mv "$stage/FlovaCutter-mac.zip" dist/FlovaCutter-mac.zip

GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o dist/FlovaCutter.exe .
