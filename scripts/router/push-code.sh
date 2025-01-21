#!/usr/bin/env bash

# Push code locally to the router
set -euxo pipefail

source ./secrets/router-params.sh

rsync -zv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY" \
    --exclude=.git \
    --exclude=.direnv \
    . \
    $ROUTER_USER@$ROUTER_IP:/share/repo/branch-by-branch