#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./rclone-push.sh meta-llama/Llama-3.1-8B-Instruct path-to-model-on-drive

    push model to b2 

    expects ../secrets/rclone.conf to exist.
'
    exit
fi

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

MODEL_NAME="$1"
DRIVE_PATH="$2"
rclone --progress --transfers 12 --config "$script_dir/../secrets/rclone.conf" copy "$DRIVE_PATH" "b2:branch-by-branch/models/$MODEL_NAME"


# upload back
# rclone --progress --transfers 12 --config ./branch-by-branch/secrets/rclone.conf copy ./bas70 "b2:branch-by-branch/llama70b-02-11-1"

