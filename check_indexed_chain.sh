#!/bin/bash

# bitcoin
BTC_VERSION=$(curl -s https://api.github.com/repos/bitcoin/bitcoin/releases/latest | grep "tag_name" | awk '{print substr($2, 3, length($2)-4)}')
echo bitcoin-core=${BTC_VERSION}
sed -i "s/BTCD_VERSION=.*/BTCD_VERSION=${BTC_VERSION}/" docker/Dockerfile.bitcoin-core
# ethereum
ETH_VERSION=$(curl -s https://api.github.com/repos/ethereum/go-ethereum/releases/latest | grep "tag_name" | awk '{print substr($2, 3, length($2)-4)}')
ETH_SHA=$(curl -s https://api.github.com/repos/ethereum/go-ethereum/git/ref/tags/v${ETH_VERSION} | grep "sha" | awk '{print substr($2, 2, 8)}')
echo geth=${ETH_VERSION}-${ETH_SHA}
sed -i "s/GETH_VERSION=.*/GETH_VERSION=${ETH_VERSION}-${ETH_SHA}/" docker/Dockerfile.geth
# neo
NEO2_VERSION=$(curl -s https://api.github.com/repos/neo-project/neo-node/releases | grep "tag_name" | grep "v2" | head -1 | awk '{print substr($2, 3, length($2)-4)}')
echo neo2=${NEO2_VERSION}
sed -i "s/NEO_VERSION=\".*/NEO_VERSION=\"v${NEO2_VERSION}\"/" docker/build-neo*.sh
# waves
WAVES_VERSION=$(curl -s https://api.github.com/repos/wavesplatform/Waves/releases | grep "tag_name" | grep "v1.4" | head -1 | awk '{print substr($2, 3, length($2)-4)}')
echo waves=${WAVES_VERSION}
sed -i "s/WAVES_VERSION=\".*/WAVES_VERSION=\"v${WAVES_VERSION}\"/" docker/build-waves*.sh
# lisk
LISK_VERSION=$(curl -s https://api.github.com/repos/LiskHQ/lisk-core/releases/latest | grep "tag_name" | awk '{print substr($2, 3, length($2)-4)}')
echo lisk=${LISK_VERSION}
sed -i "s/LISK_VERSION=\".*/LISK_VERSION=\"v${LISK_VERSION}\"/" docker/build-lisk*.sh
