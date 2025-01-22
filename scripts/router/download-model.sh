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


CACHE_DIR="$HOME/cache"
if [ ! -d "$CACHE_DIR" ]; then
    echo "Cache dir does not exist"
    exit 1
fi

mkdir -p "$CACHE_DIR/models/$MODEL_NAME"
rsync -rv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY -o StrictHostKeyChecking=no" \
    "$ROUTER_USER@$ROUTER_IP:/share/models/$MODEL_NAME/*" \
    "$CACHE_DIR/models/$MODEL_NAME"
