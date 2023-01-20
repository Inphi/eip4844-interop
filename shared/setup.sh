#!/bin/sh

set -e

# setup shared env vars
. ./shared.env

# These values should correspond to the CAPELLA_FORK_EPOCH and EIP4844_FORK_EPOCH in the chain config. 
# So they should be 108 for capella (CapellaForkEpoch * SecondsPerSlot * SlotsPerEpoch) and 144 for EIP-4844.
GENESIS=$(($(date +%s) + 60))   # 60s till genesis
SHANGHAI=$(($GENESIS + 108))    # 108s till shanghai
CANCUN=$(($GENESIS + 144))      # 144s till cancun

# generate new genesis with updated time
cp genesis.json generated-genesis.json
sed -i -e 's/GENESIS_TIME/'$GENESIS'/'      /shared/generated-genesis.json
sed -i -e 's/SHANGHAI_TIME/'$SHANGHAI'/'    /shared/generated-genesis.json
sed -i -e 's/SHARDING_FORK_TIME/'$CANCUN'/' /shared/generated-genesis.json

# prysmctl is built by Dockerfile.shared, if you want to execute this locally, do build it yourself
/usr/local/bin/prysmctl \
    testnet \
    generate-genesis \
    --num-validators 4 \
    --output-ssz=/shared/generated-genesis.ssz \
    --chain-config-file=/shared/chain-config.yml \
    --genesis-time=$GENESIS

# append genesis time to shared env vars for CL / EL to reference during startup
cp shared.env generated-shared.env # start from clean
echo GENESIS=$GENESIS >> generated-shared.env