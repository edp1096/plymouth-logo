#!/bin/sh
set -eu
cd "$(dirname "$0")"
exec ./dist/plymouth-logo --install "$@"
