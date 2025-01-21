#!/usr/bin/env bash

# Push code locally to the router
set -euxo pipefail

rsync -zv \
    --info=progress2 \
    -e "ssh -i ./secrets/hetzner" \
    --exclude=.git \
    --exclude=.direnv \
    . \
    root@/share/repo/branch-by-branch