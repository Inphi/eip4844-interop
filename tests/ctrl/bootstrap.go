package ctrl

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	types "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	beaconservice "github.com/prysmaticlabs/prysm/v3/proto/eth/service"
	ethpbv1 "github.com/prysmaticlabs/prysm/v3/proto/eth/v1"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

var env *TestEnvironment

func InitE2ETest() {
	ctx := context.Background()
	if err := StopDevnet(); err != nil {
		log.Fatalf("unable to stop devnet: %v", err)
	}
	env := GetEnv()
	env.StartAll(ctx)
}

func GetEnv() *TestEnvironment {
	if env == nil {
		env = newTestEnvironment()
	}
	return env
}

func WaitForShardingFork() {
	ctx := context.Background()

	config := env.GethChainConfig
	eip4844ForkBlock := config.ShardingForkBlock.Uint64()

	stallTimeout := 1 * time.Minute

	client, err := GetExecutionClient(ctx)
	if err != nil {
		log.Fatalf("unable to retrive beacon node client: %v", err)
	}

	log.Printf("waiting for sharding fork block...")
	var lastBn uint64
	var lastUpdate time.Time
	for {
		bn, err := client.BlockNumber(ctx)
		if err != nil {
			log.Fatalf("ethclient.BlockNumber: %v", err)
		}
		if bn >= eip4844ForkBlock {
			break
		}
		// Chain stall detection
		if bn != lastBn {
			lastBn = bn
			lastUpdate = time.Now()
		} else if time.Since(lastUpdate) > stallTimeout {
			log.Fatalf("Chain is stalled on block %v", bn)
		}
		time.Sleep(time.Second * 1)
	}
}

func ReadGethChainConfig() *params.ChainConfig {
	path := shared.GethChainConfigFilepath()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("unable to read geth chain config file at %v: %v", path, err)
	}
	var genesis core.Genesis
	if err := json.Unmarshal(data, &genesis); err != nil {
		log.Fatalf("invalid chain config at %v: %v", path, err)
	}
	return genesis.Config
}

func ReadBeaconChainConfig() *BeaconChainConfig {
	path := shared.BeaconChainConfigFilepath()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("unable to read beacon chain config file at %v: %v", path, err)
	}
	var config BeaconChainConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Fatalf("invalid beacon chain config file at %v: %v", path, err)
	}
	return &config
}

func WaitForSlot(ctx context.Context, slot types.Slot) error {
	client, err := GetBeaconNodeClient(ctx)
	if err != nil {
		return err
	}
	return WaitForSlotWithClient(ctx, client, slot)
}

func WaitForSlotWithClient(ctx context.Context, client beaconservice.BeaconChainClient, slot types.Slot) error {
	req := &ethpbv1.BlockRequest{BlockId: []byte("head")}
	for {
		header, err := client.GetBlockHeader(ctx, req)
		if err != nil {
			return fmt.Errorf("unable to retrieve block header: %v", err)
		}
		headSlot := header.Data.Header.Message.Slot
		if headSlot >= slot {
			break
		}
		time.Sleep(time.Second * time.Duration(env.BeaconChainConfig.SecondsPerSlot))
	}
	return nil
}

func WaitForEip4844ForkEpoch() {
	log.Println("waiting for eip4844 fork epoch...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	config := env.BeaconChainConfig
	eip4844Slot := config.Eip4844ForkEpoch * config.SlotsPerEpoch
	if err := WaitForSlot(ctx, types.Slot(eip4844Slot)); err != nil {
		log.Fatal(err)
	}
}

type BeaconChainConfig struct {
	AltairForkEpoch         uint64 `yaml:"ALTAIR_FORK_EPOCH"`
	BellatrixForkEpoch      uint64 `yaml:"BELLATRIX_FORK_EPOCH"`
	Eip4844ForkEpoch        uint64 `yaml:"EIP4844_FORK_EPOCH"`
	SecondsPerSlot          uint64 `yaml:"SECONDS_PER_SLOT"`
	SlotsPerEpoch           uint64 `yaml:"SLOTS_PER_EPOCH"`
	TerminalTotalDifficulty uint64 `yaml:"TERMINAL_TOTAL_DIFFICULTY"`
}

type TestEnvironment struct {
	GethChainConfig    *params.ChainConfig
	BeaconChainConfig  *BeaconChainConfig
	BeaconNode         Service
	GethNode           Service
	ValidatorNode      Service
	BeaconNodeFollower Service
	GethNode2          Service
}

func newTestEnvironment() *TestEnvironment {
	return &TestEnvironment{
		GethChainConfig:    ReadGethChainConfig(),
		BeaconChainConfig:  ReadBeaconChainConfig(),
		BeaconNode:         NewBeaconNode(),
		GethNode:           NewGethNode(),
		ValidatorNode:      NewValidatorNode(),
		BeaconNodeFollower: NewBeaconNodeFollower(),
		GethNode2:          NewGethNode2(),
	}
}

func (env *TestEnvironment) StartAll(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return env.BeaconNode.Start(ctx)
	})
	g.Go(func() error {
		return env.GethNode.Start(ctx)
	})
	g.Go(func() error {
		return env.ValidatorNode.Start(ctx)
	})
	g.Go(func() error {
		return env.BeaconNodeFollower.Start(ctx)
	})
	g.Go(func() error {
		return env.GethNode2.Start(ctx)
	})
	return g.Wait()
}
