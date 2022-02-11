#!/bin/bash

GITHASH=$(git rev-parse --short HEAD)

docker build -t local/gool:${GITHASH} -t local/gool:latest -f docker/Dockerfile.gool .
docker build -t local/geth:${GITHASH} -t local/geth:latest -f docker/Dockerfile.geth .
docker build -t local/bitcoin-core:${GITHASH} -t local/bitcoin-core:latest -f docker/Dockerfile.bitcoin-core .
docker/build-waves-native.sh local ${GITHASH}
docker/build-neo-native.sh local ${GITHASH}
docker/build-lisk-native.sh local ${GITHASH}
