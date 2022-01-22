#!/bin/bash

WORKDIR=$1

if [ -z $WORKDIR ]
then
    WORKDIR=$(pwd)
fi

BASE=${WORKDIR}/.overline

mkdir -p ${BASE}
mkdir -p ${BASE}/data/genesis

gunzip -c data/genesis/at_genesis_balances.csv.gz > ${BASE}/data/genesis/at_genesis_balances.csv
