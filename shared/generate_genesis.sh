#!/bin/sh

GENESIS=$(($(date +%s) + 60))   # 60s till genesis
SHANGHAI=$(($GENESIS + 108))    # 108s till shanghai
CANCUN=$(($GENESIS + 144))      # 144s till cancun

sed -i -e 's/XXX/'$SHANGHAI'/'  ./genesis.json
sed -i -e 's/YYY/'$CANCUN'/'    ./genesis.json
