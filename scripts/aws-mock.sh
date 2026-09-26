#!/bin/sh
# Minimal S3 shim for CI: rewrites s3:// URIs to /tmp/s3/ and copies.
if [ "$1" = "s3" ] && [ "$2" = "cp" ]; then
  src="$3"
  dest="$4"
  case "$src" in s3://*) src="/tmp/s3/${src#s3://}" ;; esac
  case "$dest" in s3://*) dest="/tmp/s3/${dest#s3://}" ;; esac
  mkdir -p "$(dirname "$dest")"
  cp "$src" "$dest"
fi
