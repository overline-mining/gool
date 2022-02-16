#!/bin/bash

NEOVERSION=$1
PLUGINS="ApplicationLogs ImportBlocks RpcWallet SimplePolicy CoreMetrics RpcSystemAssetTracker RpcNep5Tracker RpcSecurity StatesDumper"

for plugin in $PLUGINS
do
    curl -Ls https://github.com/neo-project/neo-modules/releases/download/${NEOVERSION}/${plugin}.zip -o ${plugin}.zip
    unzip -qq ${plugin}.zip
    rm ${plugin}.zip
done
