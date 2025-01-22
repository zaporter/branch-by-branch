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
python3 -m venv env
source ./env/bin/activate

echo "python:"
which python

pip install -r requirements.txt

# source secrets
source ../.env

python ./inference.py
