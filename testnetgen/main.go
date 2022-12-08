package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/eth2/testnet/setup"
	"github.com/holiman/uint256"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/codec"
	"github.com/protolambda/ztyp/view"
)

var (
	TERMINAL_TOTAL_DIFFICULTY        = big.NewInt(2)
	ALTAIR_FORK_EPOCH                = 0
	MERGE_FORK_EPOCH                 = 0
	VALIDATOR_COUNT           uint64 = 16
	SLOT_TIME                        = 6
)

var (
	depositAddress     common.Eth1Address
	fundAddress        = gethCommon.HexToAddress("a94f5374fce5edbc8e2a8697c15331677e6ebf0b")
	fundAccountBalance *big.Int
)

func init() {
	_ = depositAddress.UnmarshalText([]byte("0x4242424242424242424242424242424242424242"))
	fundAccountBalance = new(big.Int)
	fundAccountBalance.SetString("0x6d6172697573766477000000", 0)
}

var (
	genesisOutput   = flag.String("genesis-output", "", "path to output generated genesis file")
	keysOutput      = flag.String("keys-output", "", "directory to keys output")
	eth1GenesisFile = flag.String("eth1-genesis", "", "path to output generated eth1 genesis")
)

func main() {
	flag.Parse()

	if *genesisOutput == "" {
		log.Fatal("missing --genesis-output flag")
	}
	if *keysOutput == "" {
		log.Fatal("missing --keys-output flag")
	}
	if *eth1GenesisFile == "" {
		log.Fatal("missing --eth1-genesis flag")
	}

	state := GenerateGenesisBeaconState()
	var buf bytes.Buffer
	if err := state.Serialize(codec.NewEncodingWriter(&buf)); err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(*genesisOutput, buf.Bytes(), 0644); err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(fmt.Sprintf("%s/secrets", *keysOutput), os.ModePerm); err != nil {
		log.Fatal(err)
	}

	keys := GenerateKeys()
	for i := range keys {
		basedir := fmt.Sprintf("%s/keystores/0x%x", *keysOutput, keys[i].ValidatorPubkey[:])
		if err := os.MkdirAll(basedir, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		p := fmt.Sprintf("%s/keystore.json", basedir)
		if err := ioutil.WriteFile(p, keys[i].ValidatorKeystoreJSON, 0644); err != nil {
			log.Fatal(err)
		}

		p = fmt.Sprintf("%s/secrets/0x%x", *keysOutput, keys[i].ValidatorPubkey[:])
		if err := ioutil.WriteFile(p, []byte(keys[i].ValidatorKeystorePass), 0644); err != nil {
			log.Fatal(err)
		}
	}
}

func GenerateKeys() []*setup.KeyDetails {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"
	keySrc := &setup.MnemonicsKeySource{
		From:       0,
		To:         VALIDATOR_COUNT,
		Validator:  mnemonic,
		Withdrawal: mnemonic,
	}
	keys, err := keySrc.Keys()
	if err != nil {
		log.Fatal(err)
	}
	return keys
}

func GenerateGenesisBeaconState() common.BeaconState {
	eth1GenesisTime := common.Timestamp(time.Now().Unix())
	eth2GenesisTime := eth1GenesisTime + 30

	// Generate genesis for execution clients
	eth1Genesis := setup.BuildEth1Genesis(TERMINAL_TOTAL_DIFFICULTY, uint64(eth1GenesisTime), true)
	eth1Genesis.Genesis.Config.Clique.Period = 14
	eth1Genesis.Genesis.Config.Clique.Epoch = 30000
	// Disable MergeForkBlock while relying on clique mining to reach ttd
	eth1Genesis.Genesis.Config.MergeForkBlock = nil
	eth1Genesis.Genesis.ExtraData = gethCommon.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000000a94f5374fce5edbc8e2a8697c15331677e6ebf0b0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")

	eth1Genesis.Genesis.Alloc[fundAddress] = core.GenesisAccount{Balance: fundAccountBalance}

	eth1GenesisJSON, err := json.Marshal(eth1Genesis.Genesis)
	if err != nil {
		log.Fatal(err)
	}
	// Hack to set Config.TerminalTotalDifficultyPassed and ShardingForkBlock because the hive geth dependency doesn't yet support these fields
	jason := make(map[string]interface{})
	if err := json.Unmarshal(eth1GenesisJSON, &jason); err != nil {
		log.Fatal(err)
	}
	v := jason["config"].(map[string]interface{})
	v["terminalTotalDifficultyPassed"] = true
	v["shardingForkBlock"] = 10 // (EIP4844_FORK_EPOCH - BELLATRIX_FORK_EPOCH) * SLOTS_PER_EPOCH + TTD
	eth1GenesisJSON, err = json.Marshal(jason)
	if err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(*eth1GenesisFile, eth1GenesisJSON, 0644); err != nil {
		log.Fatal(err)
	}

	specCpy := *configs.Mainnet
	spec := &specCpy
	spec.Config.DEPOSIT_CONTRACT_ADDRESS = depositAddress
	spec.Config.DEPOSIT_CHAIN_ID = eth1Genesis.Genesis.Config.ChainID.Uint64()
	spec.Config.DEPOSIT_NETWORK_ID = eth1Genesis.NetworkID
	spec.Config.ETH1_FOLLOW_DISTANCE = 1

	spec.Config.MIN_GENESIS_ACTIVE_VALIDATOR_COUNT = VALIDATOR_COUNT
	spec.Config.SECONDS_PER_SLOT = common.Timestamp(SLOT_TIME)
	tdd, _ := uint256.FromBig(TERMINAL_TOTAL_DIFFICULTY)
	spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)

	// avoid collisoins with mainnet/eip4844 config versioning
	spec.Config.GENESIS_FORK_VERSION = common.Version{0x00, 0x00, 0x0f, 0xfe}
	// Commented out beacuse prysm doesn't yet support post-merge genesis
	//spec.Config.ALTAIR_FORK_VERSION = common.Version{0x01, 0x00, 0x0f, 0xfe}
	//spec.Config.BELLATRIX_FORK_VERSION = common.Version{0x02, 0x00, 0x0f, 0xfe}
	//spec.Config.EIP4844_FORK_VERSION = common.Version{0x03, 0x00, 0x0f, 0xfe}

	keys := GenerateKeys()
	state, err := setup.BuildBeaconState(spec, eth1Genesis.Genesis, eth2GenesisTime, keys)
	if err != nil {
		log.Fatal(err)
	}

	return state
}
