#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./rclone-model.sh meta-llama/Llama-3.1-8B-Instruct

    download model from b2 

    expects ../secrets/rclone.conf to exist.
'
    exit
fi

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
pushd "$script_dir"

MODEL_NAME="$1"

mkdir -p "cache/models/$MODEL_NAME"
rclone --progress --transfers 12 --config ../secrets/rclone.conf copy "b2:branch-by-branch/$MODEL_NAME" "$HOME/cache/models/$MODEL_NAME"


# upload back
# rclone --progress --transfers 12 --config ./branch-by-branch/secrets/rclone.conf copy ./bas70 "b2:branch-by-branch/llama70b-02-11-1"

