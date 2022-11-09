#!/bin/sh

: "${EXECUTION_NODE_URL:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${VERBOSITY:-info}"

# wait for the execution node to start
RETRIES=60
i=0
until curl --silent --fail "$EXECUTION_NODE_URL";
do
    sleep 1
    if [ $i -eq $RETRIES ]; then
        echo 'Timed out waiting for execution node'
        exit 1
    fi
    echo 'Waiting for execution node...'
    ((i=i+1))
done

echo 'Execution client running. Starting Lodestar beacon node.'

# https://chainsafe.github.io/lodestar/usage/local/
./lodestar dev \
    --logLevel verbose \
    --paramsFile /config/chain-config.yml \
    --genesisValidators 1 \
    --startValidators 0..1 \
    --enr.ip 127.0.0.1 \
    --server http://localhost:3500 \
    --reset \
    --eth1 \
    --eth1.providerUrls="$EXECUTION_NODE_URL" \
    --execution.urls="$EXECUTION_NODE_URL" \
    --dataDir /chaindata \
    --rest \
    --rest.address 0.0.0.0 \
    --rest.port 3500 \
    --rest.namespace "*" \
    --metrics \
    --logFile /logs/beacon.log \
    --logFileLevel debug \
    --suggestedFeeRecipient 0x8A04d14125D0FDCDc742F4A05C051De07232EDa4
