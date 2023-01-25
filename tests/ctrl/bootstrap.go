package ctrl

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/util"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
	types "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
)

var consensusClientEnvironment *TestEnvironment

// Stateful. InitE2ETest sets this.
var client string

func GetEnv() *TestEnvironment {
	return consensusClientEnvironment
}

func InitEnvForClient(clientName string) *TestEnvironment {
	client = clientName
	switch client {
	case "prysm":
		consensusClientEnvironment = newPrysmTestEnvironment()
	case "lodestar":
		consensusClientEnvironment = newLodestarTestEnvironment()
	case "lighthouse":
		consensusClientEnvironment = newLighthouseTestEnvironment()
	default:
		log.Fatalf("unknown client %s", clientName)
	}
	return consensusClientEnvironment
}

func InitE2ETest(clientName string) {
	ctx := context.Background()
	if err := StopDevnet(); err != nil {
		log.Fatalf("unable to stop devnet: %v", err)
	}

	env := InitEnvForClient(clientName)
	if err := env.StartAll(ctx); err != nil {
		log.Fatalf("unable to start environment: %v", err)
	}
}

func WaitForShardingFork() {
	ctx := context.Background()

	config := GetEnv().GethChainConfig
	eip4844ForkTime := config.ShardingForkTime
	if eip4844ForkTime == nil {
		log.Fatalf("shardingForkTime is not set in configuration")
	}

	stallTimeout := 60 * time.Minute

	client, err := GetExecutionClient(ctx)
	if err != nil {
		log.Fatalf("unable to retrive beacon node client: %v", err)
	}

	log.Printf("waiting for sharding fork time...")
	var lastBn uint64
	lastUpdate := time.Now()
	for {
		b, err := client.BlockByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("ethclient.BlockByNumber: %v", err)
		}
		if b.Time() >= *eip4844ForkTime {
			break
		}
		// Chain stall detection
		if b.NumberU64() != lastBn {
			lastBn = b.NumberU64()
			lastUpdate = time.Now()
		} else if time.Since(lastUpdate) > stallTimeout {
			log.Fatalf("Chain is stalled on block %v", b.NumberU64())
		}
		time.Sleep(time.Second * 1)
	}
}

func ReadGethChainConfig() *params.ChainConfig {
	return ReadGethChainConfigFromPath(shared.GethChainConfigFilepath())
}

func ReadGethChainConfigFromPath(path string) *params.ChainConfig {
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
	return ReadBeaconChainConfigFromPath(shared.BeaconChainConfigFilepath())
}

func ReadBeaconChainConfigFromPath(path string) *BeaconChainConfig {
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

func WaitForSlotWithClient(ctx context.Context, client *beacon.Client, slot types.Slot) error {
	for {
		headSlot := util.GetHeadSlot(ctx, client)
		if headSlot >= slot {
			break
		}
		time.Sleep(time.Second * time.Duration(GetEnv().BeaconChainConfig.SecondsPerSlot))
	}
	return nil
}

func WaitForEip4844ForkEpoch() {
	log.Println("waiting for eip4844 fork epoch...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	config := GetEnv().BeaconChainConfig
	// TODO: query /eth/v1/config/spec for time parameters
	eip4844Slot := config.Eip4844ForkEpoch * config.SlotsPerEpoch
	if err := WaitForSlot(ctx, types.Slot(eip4844Slot)); err != nil {
		log.Fatal(err)
	}
}

type BeaconChainConfig struct {
	AltairForkEpoch         uint64 `yaml:"ALTAIR_FORK_EPOCH"`
	BellatrixForkEpoch      uint64 `yaml:"BELLATRIX_FORK_EPOCH"`
	Eip4844ForkEpoch        uint64 `yaml:"EIP4844_FORK_EPOCH"`
	SlotsPerEpoch           uint64 `yaml:"SLOTS_PER_EPOCH"`
	SecondsPerSlot          uint64 `yaml:"SECONDS_PER_SLOT"`
	TerminalTotalDifficulty uint64 `yaml:"TERMINAL_TOTAL_DIFFICULTY"`
	BellatrixForkVersion    string `yaml:"BELLATRIX_FORK_VERSION"`
	CapellaForkVersion      string `yaml:"CAPELLA_FORK_VERSION"`
	EIP4844ForkVersion      string `yaml:"EIP4844_FORK_VERSION"`
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

func setupGeneratedConfigs() {
	if err := StartServices("genesis-generator"); err != nil {
		log.Fatalf("failed to start genesis-generator service: %v", err)
	}
	// TODO: it takes a moment for the docker daemon to synchronize files
	time.Sleep(time.Second * 10)
}

func newPrysmTestEnvironment() *TestEnvironment {
	setupGeneratedConfigs()

	shared.BeaconAPI = "localhost:3500"
	shared.BeaconFollowerAPI = "localhost:3501"

	clientName := "prysm"
	return &TestEnvironment{
		BeaconChainConfig:  ReadBeaconChainConfig(),
		BeaconNode:         NewBeaconNode(clientName),
		BeaconNodeFollower: NewBeaconNodeFollower(clientName),
		ValidatorNode:      NewValidatorNode(clientName),
		GethChainConfig:    ReadGethChainConfig(),
		GethNode:           NewGethNode(),
		GethNode2:          NewGethNode2(),
	}
}

func newLodestarTestEnvironment() *TestEnvironment {
	clientName := "lodestar"
	return &TestEnvironment{
		BeaconChainConfig:  ReadBeaconChainConfig(),
		BeaconNode:         NewBeaconNode(clientName),
		BeaconNodeFollower: NewBeaconNodeFollower(clientName),
		GethChainConfig:    ReadGethChainConfig(),
		GethNode:           NewGethNode(),
		GethNode2:          NewGethNode2(),
	}
}

func newLighthouseTestEnvironment() *TestEnvironment {
	setupGeneratedConfigs()

	clientName := "lighthouse"
	return &TestEnvironment{
		BeaconChainConfig:  ReadBeaconChainConfig(),
		BeaconNode:         NewBeaconNode(clientName),
		BeaconNodeFollower: NewBeaconNodeFollower(clientName),
		ValidatorNode:      NewValidatorNode(clientName),
		GethChainConfig:    ReadGethChainConfig(),
		GethNode:           NewGethNode(),
		GethNode2:          NewGethNode2(),
	}
}

func (env *TestEnvironment) StartAll(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return env.BeaconNode.Start(ctx)
	})
	g.Go(func() error {
		if env.ValidatorNode != nil {
			return env.ValidatorNode.Start(ctx)
		}
		return nil
	})
	g.Go(func() error {
		if env.BeaconNodeFollower != nil {
			return env.BeaconNodeFollower.Start(ctx)
		}
		return nil
	})
	g.Go(func() error {
		return env.GethNode.Start(ctx)
	})
	g.Go(func() error {
		if env.GethNode2 != nil {
			return env.GethNode2.Start(ctx)
		}
		return nil
	})
	return g.Wait()
}
