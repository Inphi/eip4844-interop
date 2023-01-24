## Setup
1. Clone this repository
2. Run `git submodule update --init`

Everytime you need to test a change in prysm or geth, run `git submodule update --remote`

## Running the Devnet
1. (_Optional_) Run `make devnet-clean` to start from a clean slate
2. Run `make devnet-up`
3. Visit http://127.0.0.1:16686 to visualize beacon and validator node traces

## Adding new Clients
Interop uses [ethereum-genesis-generator](https://github.com/inphi/ethereum-genesis-generator/tree/eip4844) to generate the configuration.
New clients can be added by create a docker compose service running the client. Recommend taking a look at the existing docker compose services to get an idea.

The `genesis-generator` docker service creates the genesis configuration your client will need to run a local testnet. The configs live in the `config_data` volume with the following layout:

```
/config_data
├── cl
│   └── jwtsecret
├── custom_config_data
│   ├── besu.json
│   ├── boot_enr.txt
│   ├── boot_enr.yaml
│   ├── bootstrap_nodes.txt
│   ├── chainspec.json
│   ├── config.yaml
│   ├── deploy_block.txt
│   ├── deposit_contract.txt
│   ├── deposit_contract_block.txt
│   ├── deposit_contract_block_hash.txt
│   ├── genesis.json
│   ├── genesis.ssz
│   ├── mnemonics.yaml
│   ├── parsedBeaconState.json
│   └── tranches
│       └── tranche_0000.txt
└── el
    └── jwtsecret
```
The generated CL configs contain the following noteworthy settings:
- `GENESIS_TIMESTAMP`: this is set to the current time
- `GENESIS_DELAY`: this is set to 60 seconds, giving clients a minute to build and run their nodes before genesis begins.
- `SECONDS_PER_SLOT`: set to `3` to shorten test iteration.

For consensus clients, you can use the existing `geth-x` docker services to build blocks. Once EIP4844 epoch has occurred, you can try sending a blob transaction locally to confirm that the blobs are sidecar'd to the beacon chain.

