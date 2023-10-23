#!/bin/bash

WAVES_VERSION="v1.4.19"
ARCH=$1
REPO_NAME=$2
GOOL_VERSION=$3
DO_PUSH=$4

mkdir -p $(pwd)/waves-build
pushd $(pwd)/waves-build

git clone https://github.com/wavesplatform/Waves.git -b ${WAVES_VERSION}
pushd Waves

#sed -i 's/buster/unstable/' docker/Dockerfile
sed -i 's/-it//' build-with-docker.sh
./build-with-docker.sh && docker buildx build --platform ${ARCH} --tag ${REPO_NAME}/waves:${GOOL_VERSION} docker ${DO_PUSH} || { echo "WAVES BUILD FAILED"; popd; popd; rm -rf $(pwd)/waves-build; exit 1; }

popd

popd
#rm -rf $(pwd)/waves-build
