#!/bin/env bash

set -exu

: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${TRACING_ENDPOINT:-}"
: "${VERBOSITY:-info}"

beacon-node \
    --accept-terms-of-use \
    --verbosity="$VERBOSITY" \
    --datadir /chaindata \
    --force-clear-db \
    --interop-eth1data-votes \
    --http-web3provider="$EXECUTION_NODE_URL" \
    --deposit-contract 0x8A04d14125D0FDCDc742F4A05C051De07232EDa4 \
    --bootstrap-node= \
    --chain-config-file=/config/prysm-chain-config.yml \
    --disable-sync \
    --contract-deployment-block 0 \
    --interop-num-validators 4 \
    --rpc-host 0.0.0.0 \
    --rpc-port 4000 \
    --grpc-gateway-host 0.0.0.0 \
    --grpc-gateway-port 3500 \
    --enable-debug-rpc-endpoints \
    --enable-tracing \
    --tracing-endpoint "$TRACING_ENDPOINT" \
    --tracing-process-name "$PROCESS_NAME" $@
