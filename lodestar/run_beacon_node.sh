#!/bin/sh

: "${EXECUTION_NODE_URL:-}"
: "${VERBOSITY:-info}"

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
    --suggestedFeeRecipient 0x8A04d14125D0FDCDc742F4A05C051De07232EDa4
