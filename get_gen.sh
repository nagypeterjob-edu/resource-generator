#!/bin/sh
set -e

TAR_FILE="/tmp/gen.tar.gz"
RELEASES_URL="https://github.com/nagypeterjob-edu/resource-generator/releases"
test -z "$TMPDIR" && TMPDIR="$(mktemp -d)"
echo "tmp dir: ${TMPDIR}"

last_version() {
  curl -sL -o /dev/null -w %{url_effective} "$RELEASES_URL/latest" | 
    rev | 
    cut -f1 -d'/'| 
    rev
}

download() {
  test -z "$VERSION" && VERSION="$(last_version)"
  test -z "$VERSION" && {
    echo "Unable to get resource-generator version." >&2
    exit 1
  }
  rm -f "$TAR_FILE"

  echo "Downloading: $RELEASES_URL/download/$VERSION/resource-generator_$(uname -s)_$(uname -m).tar.gz"
  curl -s -L -o "$TAR_FILE" \
    "$RELEASES_URL/download/$VERSION/resource-generator_$(uname -s)_$(uname -m).tar.gz"
}

download
tar -xf "$TAR_FILE" -C "$TMPDIR"
echo "Unzipped: $TAR_FILE to $TMPDIR"
mv "$TMPDIR/gen" $(pwd)
