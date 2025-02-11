#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
if [[ "${TRACE-0}" == "1" ]]; then
    set -o xtrace
fi

if [[ "${1-}" =~ ^-*h(elp)?$ ]]; then
    echo 'Usage: ./lambda-mkswap.sh {size}

    Ex: ./lambda-mkswap.sh 200G

    (Reincarnation as a lambda labs ssd is the 10th circle of hell -- this is mostly used to load, quantize, & re-save large models)
'
    exit
fi


sudo fallocate -l "$1" /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
sudo swapon --show
