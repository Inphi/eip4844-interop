#!/bin/sh

set -exu -o pipefail

source /config/vars.env

VERBOSITY=${GETH_VERBOSITY:-4}
GETH_DATA_DIR=/db
GETH_GENESIS=/shared-configs/genesis.json
RPC_PORT=8545
AUTH_PORT=8551
WS_PORT=8546

EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

#BOOTNODE_IP=$(host execution-bootnode | cut -d ' ' -f4)
#echo "Located bootnode at $BOOTNODE_IP"
#BOOTNODE_ENODE=$(echo $EL_BOOTNODE_ENODE | sed s"/127.0.0.1/$BOOTNODE_IP/")

#BOOTNODE_ENODE="enode://51ea9bb34d31efc3491a842ed13b8cab70e753af108526b57916d716978b380ed713f4336a80cdb85ec2a115d5a8c0ae9f3247bed3c84d3cb025c6bab311062c@192.168.192.3:30303?discport=30301"

#BOOTONODE_ENODE="enode://51ea9bb34d31efc3491a842ed13b8cab70e753af108526b57916d716978b380ed713f4336a80cdb85ec2a115d5a8c0ae9f3247bed3c84d3cb025c6bab311062c@192.168.128.2:0?discport=30301"

geth init --datadir $GETH_DATA_DIR $GETH_GENESIS

echo "Completed init"

exec geth \
    --datadir "$GETH_DATA_DIR" \
    --verbosity "$VERBOSITY" \
    --networkid "$CHAIN_ID" \
    --http \
    --http.corsdomain="*" \
    --http.vhosts="*" \
    --http.addr=0.0.0.0 \
    --http.port="$RPC_PORT" \
    --http.api=web3,debug,engine,eth,net,txpool \
    --authrpc.addr=0.0.0.0 \
    --authrpc.vhosts="*" \
    --authrpc.jwtsecret=/config/jwtsecret \
    --authrpc.port=${AUTH_PORT} \
    --ws \
    --ws.addr=0.0.0.0 \
    --ws.port="$WS_PORT" \
    --ws.origins="*" \
    --ws.api=debug,eth,txpool,net,engine \
    --nat extip:${EXTERNAL_IP} \
    --password "${GETH_DATA_DIR}/password" \
    --syncmode=full
