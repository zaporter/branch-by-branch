#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# repo root
cd "$script_dir/../.."

source ./secrets/router-params.sh

MODEL_NAME="$1"

CACHE_DIR=""
if [ -d "$HOME/cache-w2" ]; then
    CACHE_DIR="$HOME/cache-w2"
elif [ -d "$HOME/cache-w1" ]; then
    CACHE_DIR="$HOME/cache-w1"
elif [ -d "$HOME/cache-e1" ]; then
    CACHE_DIR="$HOME/cache-e1"
else
    echo "Cache dir does not exist"
    exit 1
fi


if [ ! -d "$CACHE_DIR/models/$MODEL_NAME" ]; then
    echo "Model is not cached in lambda, downloading to cache"
    mkdir -p "$CACHE_DIR/models/$MODEL_NAME"
    rsync -rv \
        --info=progress2 \
        -e "ssh -i $ROUTER_SSH_KEY -o StrictHostKeyChecking=no" \
        "$ROUTER_USER@$ROUTER_IP:/share/models/$MODEL_NAME/*" \
        "$CACHE_DIR/models/$MODEL_NAME"
fi

# if the symlink to the models dir does not exist, create it
if [ ! -d "$HOME/models" ]; then
    echo "Symlinking models dir"
    ln -s "$CACHE_DIR/models" "$HOME/models"
fi
