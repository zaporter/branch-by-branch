#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./lambda-cache-tmpfs.sh [size]
    builds tmpfs at $HOME/cache with size [size] ex: 10G
'
    exit
fi

ramfs_dir="$HOME/cache"
ramfs_size="$1"

mkdir -p "$ramfs_dir"

sudo mount -t tmpfs none "$ramfs_dir" -o size="$ramfs_size"

echo "tmpfs mounted at $ramfs_dir"
