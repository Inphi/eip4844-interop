#!/bin/sh

priv_key="02fd74636e96a8ffac8e7b01b0de8dea94d6bcf4989513b38cf59eb32163ff91"
ip addr show eth0
EXTERNAL_IP=$(ip addr show eth0 | grep inet | awk '{ print $2 }' | cut -d '/' -f1)

#bootnode -nat extip:${EXTERNAL_IP} -nodekeyhex $priv_key
bootnode -nodekeyhex $priv_key -addr ${EXTERNAL_IP}:30301 -verbosity 9
