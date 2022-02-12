#!/bin/bash

dotnet neo-cli.dll <<EOF
install ApplicationLogs
install ImportBlocks
install RpcWallet
install SimplePolicy
install CoreMetrics
install RpcSystemAssetTracker
install RpcNep5Tracker
install RpcSecurity
install StatesDumper
exit
EOF
