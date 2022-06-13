#!/bin/sh

set -exu

VERBOSITY=${GETH_VERBOSITY:-3}
GETH_DATA_DIR=/db
GETH_KEYSTORE_DIR="$GETH_DATA_DIR/keystore"
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"
GENESIS_FILE_PATH="${GENESIS_FILE_PATH:-/genesis.json}"
BLOCK_SIGNER_PRIVATE_KEY="45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
BLOCK_SIGNER_ADDRESS="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"
RPC_PORT="${RPC_PORT:-8545}"
WS_PORT="${WS_PORT:-8546}"

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

if [ ! -d "$GETH_CHAINDATA_DIR" ]; then
    echo "$GETH_CHAINDATA_DIR missing, running init"
    geth --verbosity="$VERBOSITY" init \
        --datadir "$GETH_DATA_DIR" \
        "$GENESIS_FILE_PATH"
else
    echo "$GETH_CHAINDATA_DIR exists"
fi

exec geth \
    --datadir ${GETH_DATA_DIR} \
    --verbosity ${VERBOSITY} \
    --http \
    --http.corsdomain="*" \
    --http.vhosts="*" \
    --http.addr=0.0.0.0 \
    --http.port="$RPC_PORT" \
    -http.api=web3,debug,engine,eth,net,txpool \
    --ws \
    --ws.addr=0.0.0.0 \
    --ws.port="$WS_PORT" \
    --ws.origins="*" \
    --ws.api=debug,eth,txpool,net,engine \
    --allow-insecure-unlock \
    --unlock "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" \
    --password ${GETH_DATA_DIR}/password \
    --nodiscover \
    --maxpeers=1 \
    --authrpc.jwtsecret=0x98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4 \
    --mine
