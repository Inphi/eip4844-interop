#!/bin/sh

set -exu -o pipefail

VERBOSITY=${GETH_VERBOSITY:-4}
GETH_DATA_DIR=/db
GETH_KEYSTORE_DIR="$GETH_DATA_DIR/keystore"
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"
GENESIS_FILE_PATH="${GENESIS_FILE_PATH:-/genesis.json}"
BLOCK_SIGNER_PRIVATE_KEY="45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
#BLOCK_SIGNER_ADDRESS="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"
RPC_PORT="${RPC_PORT:-8545}"
WS_PORT="${WS_PORT:-8546}"
BOOTNODE_KEY_HEX=${BOOTNODE_KEY_HEX:-65f77f40c167b52b5cc70fb33582aecbdcd81062dc1438df00a3099a07079204}
NETWORKID=69
ENABLE_MINING=true

PEER=${PEER:-}

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
    # TODO: temporary solution for generating genesis, remove this before merge
    # wait for the genesis to be updated
    RETRIES=60
    i=0
    until [ $(jq .config.shanghaiTime $GENESIS_FILE_PATH) != 0 ];
    do
        sleep 1
        if [ $i -eq $RETRIES ]; then
            echo 'Timed out waiting for updated genesis'
            exit 1
        fi
        echo 'Waiting for updated genesis...'
        ((i=i+1))
    done

    echo "$GETH_CHAINDATA_DIR missing, running init"
    geth --verbosity="$VERBOSITY" init \
        --datadir "$GETH_DATA_DIR" \
        "$GENESIS_FILE_PATH"
else
    echo "$GETH_CHAINDATA_DIR exists"
fi

# by default: flags for the bootnode
# retrieve our public IP address for discovery
EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)
ADDITIONAL_FLAGS="--nodiscover --nodekeyhex ${BOOTNODE_KEY_HEX} --nat extip:${EXTERNAL_IP}"
MINE=--mine

if [ -n "$PEER" ]; then
    PEER_PUBKEY=$(bootnode -nodekeyhex "$BOOTNODE_KEY_HEX" -writeaddress)
    PEER_ENODE="enode://${PEER_PUBKEY}@${PEER}"
    echo "Configuring static node at ${PEER_ENODE}"
    ADDITIONAL_FLAGS="--bootnodes ${PEER_ENODE}"
    MINE=
    # wait a bit for the peer to come online
    sleep 4
fi

# TODO: figure out why beacon node doesn't advance when syncmode=snap
exec geth \
    --datadir "$GETH_DATA_DIR" \
    --verbosity "$VERBOSITY" \
    --networkid "$NETWORKID" \
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
    --maxpeers=2 ${ADDITIONAL_FLAGS} \
    --authrpc.jwtsecret=0x98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4 \
    --allow-insecure-unlock \
    --password "${GETH_DATA_DIR}/password" \
    --syncmode=full \
    --unlock "0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" $MINE
