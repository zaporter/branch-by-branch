#!/usr/bin/env bash

# Push models to the router
set -euxo pipefail

source ./secrets/router-params.sh

# --delete not used because local models may not be synced with all trained models
rsync -rv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY" \
    ./models/* \
    $ROUTER_USER@$ROUTER_IP:/share/models