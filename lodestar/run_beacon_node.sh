#!/bin/sh

: "${EXECUTION_NODE_URL:-}"
: "${EXECUTION_RPC:-}"
: "${PROCESS_NAME:-beacon-node}"
: "${VERBOSITY:-info}"

# wait for the execution node to start
RETRIES=60
i=0
until curl --silent --fail "$EXECUTION_RPC";
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

BOOTNODE=$(cat /config_data/custom_config_data/boot_enr.yaml | sed 's/- //')

# https://chainsafe.github.io/lodestar/usage/local/
node ./packages/cli/bin/lodestar beacon \
    --logLevel verbose \
    --paramsFile /config_data/custom_config_data/config.yaml \
    --genesisStateFile /config_data/custom_config_data/genesis.ssz \
    --dataDir /chaindata \
    --jwt-secret /config_data/cl/jwtsecret \
    --execution.urls "$EXECUTION_NODE_URL" \
    --network.connectToDiscv5Bootnodes \
    --bootnodes "$BOOTNODE" \
    --enr.ip 127.0.0.1 \
    --eth1.providerUrls="$EXECUTION_NODE_URL" \
    --rest \
    --rest.address 0.0.0.0 \
    --rest.port 3500 \
    --rest.namespace "*" \
    --metrics \
    --logFile /logs/beacon.log \
    --logFileLevel debug \
    --suggestedFeeRecipient 0x8A04d14125D0FDCDc742F4A05C051De07232EDa4
