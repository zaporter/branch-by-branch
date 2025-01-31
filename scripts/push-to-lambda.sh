#!/usr/bin/env bash

set -euxo pipefail
script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
pushd "$script_dir/.."

source ./.env

IP="$1"

rsync -zrv \
    --info=progress2 \
    -e "ssh -i $LAMBDA_KEY_PATH" \
    --delete \
    --exclude=.git \
    --exclude=inference/env \
    --exclude=compilation/env \
    --exclude=.direnv \
    --exclude=models \
    --exclude=webui \
    --exclude=lean_corelib/.lake \
    . \
    ubuntu@$IP:/home/ubuntu/branch-by-branch
