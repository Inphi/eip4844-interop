#!/bin/sh

set -exu -o pipefail

VERBOSITY=${GETH_VERBOSITY:-4}
GETH_DATA_DIR=/db
GETH_KEYSTORE_DIR="$GETH_DATA_DIR/keystore"
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"
GETH_GENESIS=/config_data/custom_config_data/genesis.json
NETWORK_ID=42424243
BLOCK_SIGNER_PRIVATE_KEY="45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
RPC_PORT=8545
AUTH_PORT=8551
WS_PORT=8546
BOOTNODE_KEY_HEX=${BOOTNODE_KEY_HEX:-65f77f40c167b52b5cc70fb33582aecbdcd81062dc1438df00a3099a07079204}
EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)
ADDITIONAL_FLAGS="--nodiscover --nodekeyhex ${BOOTNODE_KEY_HEX} --nat extip:${EXTERNAL_IP}"

mkdir -p ${GETH_DATA_DIR}

if [ ! -d "$GETH_KEYSTORE_DIR" ]; then
    echo "$GETH_KEYSTORE_DIR missing, running account import"
    touch ${GETH_DATA_DIR}/password
    echo -n "$BLOCK_SIGNER_PRIVATE_KEY" | sed 's/0x//' > "$GETH_DATA_DIR"/block-signer-key
    geth account import \
        --datadir="$GETH_DATA_DIR" \
        --password="$GETH_DATA_DIR"/password \
        "$GETH_DATA_DIR"/block-signer-key
else
    echo "$GETH_KEYSTORE_DIR exists"
fi

# init geth data
geth init --datadir $GETH_DATA_DIR $GETH_GENESIS

# TODO: figure out why beacon node doesn't advance when syncmode=snap
exec geth \
    --datadir "$GETH_DATA_DIR" \
    --verbosity "$VERBOSITY" \
    --networkid "$NETWORK_ID" \
    --nodiscover \
    --http \
    --http.corsdomain="*" \
    --http.vhosts="*" \
    --http.addr=0.0.0.0 \
    --http.port="$RPC_PORT" \
    --http.api=web3,debug,engine,eth,net,txpool \
    --authrpc.addr=0.0.0.0 \
    --authrpc.vhosts="*" \
    --authrpc.jwtsecret=/config_data/el/jwtsecret \
    --authrpc.port="$AUTH_PORT" \
    --ws \
    --ws.addr=0.0.0.0 \
    --ws.port="$WS_PORT" \
    --ws.origins="*" \
    --ws.api=debug,eth,txpool,net,engine \
    ${ADDITIONAL_FLAGS} \
    --allow-insecure-unlock \
    --password "${GETH_DATA_DIR}/password" \
    --syncmode=full \
    --unlock "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" \
    --mine
