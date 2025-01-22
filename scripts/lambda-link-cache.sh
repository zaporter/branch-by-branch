#!/usr/bin/env bash
# setup lambda labs cache-sharing.
# (it is so painful that they don't have a way to do this...)

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

CACHE_DIR=""
if [ -d "$HOME/cache-w2" ]; then
    CACHE_DIR="$HOME/cache-w2"
elif [ -d "$HOME/cache-w1" ]; then
    CACHE_DIR="$HOME/cache-w1"
elif [ -d "$HOME/cache-w3" ]; then
    CACHE_DIR="$HOME/cache-w3"
elif [ -d "$HOME/cache-e1" ]; then
    CACHE_DIR="$HOME/cache-e1"
else
    echo "Cache dir does not exist"
    exit 1
fi

echo "Cache dir: $CACHE_DIR"

ln -s "$CACHE_DIR" "$HOME/cache"