#!/bin/bash

set -e

if [ -z "${EMISSARY_IMAGE_TAG}" ]; then
    export EMISSARY_IMAGE_TAG=$(git rev-parse --short HEAD)
fi

docker-compose down
rm -rf shared conf/agent/agent.conf conf/agent/bootstrap.crt
