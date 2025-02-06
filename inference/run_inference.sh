#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

script_dir="$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
# start in inference dir
cd "$script_dir"
CACHE_DIR="foo-dont-use-lambda-fs-is-abhorrent/cache"
VENV_DIR="env"
if [[ -d "$CACHE_DIR" ]]; then
    mkdir -p "$CACHE_DIR/inference"
    VENV_DIR="$CACHE_DIR/inference/env"
fi

python3 -m venv "$VENV_DIR"

if [[ ! -f "$VENV_DIR/bin/activate" ]]; then
    echo "Failed to create virtual environment"
    exit 1
fi


source "$VENV_DIR/bin/activate"

echo "python:"
which python

# install uv if not installed
if ! which uv; then
    sudo snap install astral-uv --classic
fi

uv pip install -r requirements.txt

# source secrets
source ../.env

which python

# https://blog.vllm.ai/2025/01/27/v1-alpha-release.html
#export VLLM_USE_V1=1

python ./inference.py
