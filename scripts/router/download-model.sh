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

mkdir -p ./models/$MODEL_NAME

rsync -rv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY" \
    "$ROUTER_USER@$ROUTER_IP:/share/models/$MODEL_NAME/*" \
    ./models/$MODEL_NAME