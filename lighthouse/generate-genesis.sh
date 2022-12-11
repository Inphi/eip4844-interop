#!/bin/env bash

set -exu -o pipefail

VALIDATOR_COUNT=4
GENESIS_TIME=`date +%s`

GENESIS_FORK_VERSION=$(yq e '.GENESIS_FORK_VERSION' /config/config.yaml)

if [ ! -f "$TESTNET_DIR/genesis.ssz" ]; then
  lcli \
  	interop-genesis \
		--genesis-fork-version $GENESIS_FORK_VERSION\
  	--genesis-time $GENESIS_TIME \
  	--spec mainnet \
  	--testnet-dir $TESTNET_DIR \
  	$VALIDATOR_COUNT
fi

CAPELLA_FORK_EPOCH=$(yq e '.CAPELLA_FORK_EPOCH' /config/config.yaml)
EIP4844_FORK_EPOCH=$(yq e '.EIP4844_FORK_EPOCH' /config/config.yaml)
SECONDS_PER_SLOT=$(yq e '.SECONDS_PER_SLOT' /config/config.yaml)

GENESIS_TIME=$(lcli pretty-ssz state_merge $TESTNET_DIR/genesis.ssz  | jq | grep -Po 'genesis_time": "\K.*\d')
CAPELLA_TIME=$((GENESIS_TIME + (CAPELLA_FORK_EPOCH * 32 * SECONDS_PER_SLOT)))
EIP4844_TIME=$((GENESIS_TIME + (EIP4844_FORK_EPOCH * 32 * SECONDS_PER_SLOT)))

cp /genesis.json /genesis_data
cp $TESTNET_DIR/genesis.ssz /genesis_data

sed -i 's/"shanghaiTime".*$/"shanghaiTime": '"$CAPELLA_TIME"',/g' /genesis_data/genesis.json
sed -i 's/"shardingForkTime".*$/"shardingForkTime": '"$EIP4844_TIME"',/g' /genesis_data/genesis.json