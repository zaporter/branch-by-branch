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

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
pushd "$script_dir"

# TODO: swap entirely to UV.
python3 -m venv "env"
source "env/bin/activate"

uv pip install -r requirements.txt
cd ../..
uv run jupyter lab 


