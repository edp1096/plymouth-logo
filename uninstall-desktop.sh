#!/bin/sh
set -eu
cd "$(dirname "$0")"
if [ -x ./dist/plymouth-logo ]; then
    exec ./dist/plymouth-logo --uninstall "$@"
fi
exec "$HOME/.local/bin/plymouth-logo" --uninstall "$@"
