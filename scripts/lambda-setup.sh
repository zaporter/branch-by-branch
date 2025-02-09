#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./lambda-setup.sh 
    setup lambda
'
    exit
fi
script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
pushd "$script_dir"

sudo snap install astral-uv --classic
sudo snap install just --classic
sudo snap install rclone

# Hardcoded for 1_h100
./lambda-cache-tmpfs.sh "100G"

tmux new-session -d -s "inference" "bash $script_dir/../inference/run_inference.sh; bash"

sleep 1

tmux ls

