#!/usr/bin/env bash

set -euxo pipefail

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
    --exclude=lean_corelib/.lake \
    . \
    ubuntu@$IP:/home/ubuntu/branch-by-branch
