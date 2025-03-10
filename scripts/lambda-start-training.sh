#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./.sh 


'
    exit
fi

SETUP_FILE="/home/ubuntu/did_setup.txt"


if [ -f "$SETUP_FILE" ]; then
    echo "Setup already finished. Skipping"
    tmux ls
    exit 0
fi

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

tmux new-session -d -s "byb" "bash $script_dir/lambda-setup.sh && $script_dir/../training/run_training.sh training.py; bash"

sleep 0.1

tmux ls

echo "done" > "$SETUP_FILE"
