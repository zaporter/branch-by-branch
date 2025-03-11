#!/usr/bin/env bash

# Push code locally to the router
set -euxo pipefail

source ./secrets/router-params.sh

rsync -zrv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY" \
    --delete \
    --exclude=.git \
    --exclude=inference/env \
    --exclude=experiments \
    --exclude=etc \
    --exclude=compilation/env \
    --exclude=lean_corelib \
    --exclude=webui \
    --exclude=.direnv \
    --exclude=models \
    . \
    $ROUTER_USER@$ROUTER_IP:/share/repo/branch-by-branch
