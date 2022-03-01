#!/bin/bash

NEO_VERSION="v2.13.0"
ARCH=$1
REPO_NAME=$2
GOOL_VERSION=$3
DO_PUSH=$4

mkdir -p $(pwd)/neo-build
cp $(pwd)/docker/Dockerfile.neo $(pwd)/neo-build/
cp $(pwd)/docker/install-neo-plugins.sh $(pwd)/neo-build/
cp $(pwd)/docker/neo-config.json $(pwd)/neo-build/
pushd $(pwd)/neo-build

git clone https://github.com/neo-project/neo-node.git -b ${NEO_VERSION}
cp Dockerfile.neo neo-node/Dockerfile
cp install-neo-plugins.sh neo-node/
cp neo-config.json neo-node/
pushd neo-node

sed -i 's/netcoreapp2.1/netcoreapp3.1/' neo-cli/neo-cli.csproj

docker buildx build --platform ${ARCH} --build-arg NEO_VERSION=${NEO_VERSION} --tag ${REPO_NAME}/neo:${GOOL_VERSION} . ${DO_PUSH}

popd

popd
rm -rf $(pwd)/neo-build
