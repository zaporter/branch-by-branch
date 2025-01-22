#!/usr/bin/env bash

set -euxo pipefail

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
cd "$script_dir/../.."

source ./secrets/router-params.sh

rsync -rv \
    --info=progress2 \
    -e "ssh -i $ROUTER_SSH_KEY" \
    --delete \
    $ROUTER_USER@$ROUTER_IP:/share/repo/branch-by-branch \
    .