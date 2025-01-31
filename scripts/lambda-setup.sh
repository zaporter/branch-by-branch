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

sudo snap install astral-uv --classic
sudo snap install just --classic
sudo snap install rclone

