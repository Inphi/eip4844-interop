#!/bin/sh

set -exu -o pipefail

ERIGON_DATA_DIR=/db
ERIGON_KEYSTORE_DIR="$ERIGON_DATA_DIR/keystore"
ERIGON_CHAINDATA_DIR="$ERIGON_DATA_DIR/erigon/chaindata"
ERIGON_GENESIS=/config_data/custom_config_data/genesis.json
NETWORK_ID=42424243
BLOCK_SIGNER_PRIVATE_KEY="45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
RPC_PORT=8545
AUTH_PORT=8551
WS_PORT=8546
BOOTNODE_KEY_HEX=${BOOTNODE_KEY_HEX:-65f77f40c167b52b5cc70fb33582aecbdcd81062dc1438df00a3099a07079204}
EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)
ADDITIONAL_FLAGS="--nodiscover --nodekeyhex ${BOOTNODE_KEY_HEX} --nat extip:${EXTERNAL_IP}"

mkdir -p ${ERIGON_DATA_DIR}

# if [ ! -d "$ERIGON_KEYSTORE_DIR" ]; then
#     echo "$ERIGON_KEYSTORE_DIR missing, running account import"
#     touch ${ERIGON_DATA_DIR}/password
#     echo -n "$BLOCK_SIGNER_PRIVATE_KEY" | sed 's/0x//' > "$ERIGON_DATA_DIR"/block-signer-key
#     erigon account import \
#         --datadir="$ERIGON_DATA_DIR" \
#         --password="$ERIGON_DATA_DIR"/password \
#         "$ERIGON_DATA_DIR"/block-signer-key
# else
#     echo "$ERIGON_KEYSTORE_DIR exists"
# fi

# init erigon data
erigon init --datadir $ERIGON_DATA_DIR $ERIGON_GENESIS

# TODO: figure out why beacon node doesn't advance when syncmode=snap
exec erigon \
    --log.console.verbosity debug \
    --externalcl \
    --datadir "$ERIGON_DATA_DIR" \
    --networkid "$NETWORK_ID" \
    --nodiscover \
    --http \
    --http.api "eth,erigon,engine,debug,trace,txpool" \
    --http.corsdomain="*" \
    --http.vhosts="*" \
    --http.addr=0.0.0.0 \
    --http.port="$RPC_PORT" \
    --http.api=web3,debug,engine,eth,net,txpool \
    --authrpc.addr=0.0.0.0 \
    --authrpc.vhosts="*" \
    --authrpc.jwtsecret=/config_data/el/jwtsecret \
    --authrpc.port="$AUTH_PORT" \
    --prune=htc \
    --ws \
    ${ADDITIONAL_FLAGS}
