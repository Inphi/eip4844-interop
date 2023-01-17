#!/bin/sh

set -e

# setup shared env vars
. ./shared.env

GENESIS=$(($(date +%s) + 60))   # 60s till genesis
SHANGHAI=$(($GENESIS + 108))    # 108s till shanghai
CANCUN=$(($GENESIS + 144))      # 144s till cancun

sed -i -e 's/CHAIN_ID/'$CHAIN_ID'/'         /shared/genesis.json
sed -i -e 's/SHANGHAI_TIME/'$SHANGHAI'/'    /shared/genesis.json
sed -i -e 's/SHARDING_FORK_TIME/'$CANCUN'/' /shared/genesis.json

# prysmctl is built by Dockerfile.shared, if you want to execute this locally, do build it yourself
/usr/local/bin/prysmctl \
    testnet \
    generate-genesis \
    --num-validators 64 \
    --output-ssz=/shared/genesis.ssz \
    --chain-config-file=/shared/chain-config.yml \
    --genesis-time=$GENESIS

# append genesis time to shared env vars for CL / EL to reference during startup
echo GENESIS=$GENESIS >> shared.env