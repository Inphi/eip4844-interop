package ctrl

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	types "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	beaconservice "github.com/prysmaticlabs/prysm/v3/proto/eth/service"
	ethpbv1 "github.com/prysmaticlabs/prysm/v3/proto/eth/v1"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

func WaitForService(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	for {
		if _, err := http.Get(url); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return nil

}

func WaitForGeth(ctx context.Context) error {
	log.Printf("waiting for geth")
	return WaitForService(ctx, shared.GethRPC)
}

func WaitForBeaconNode(ctx context.Context) error {
	log.Printf("waiting for prysm beacon node")
	return WaitForService(ctx, fmt.Sprintf("%s/eth/v1/beacon/genesis", fmt.Sprintf("http://%s", shared.BeaconGatewayGRPC)))
}

func WaitForBeaconNodeFollower(ctx context.Context) error {
	log.Printf("waiting for prysm beacon node follower")
	return WaitForService(ctx, fmt.Sprintf("%s/eth/v1/beacon/genesis", shared.BeaconFollowerRPC))
}

func WaitForValidator(ctx context.Context) error {
	log.Printf("waiting for validator")
	return WaitForService(ctx, shared.ValidatorRPC)
}

func WaitForServices(ctx context.Context) error {
	if err := WaitForGeth(ctx); err != nil {
		return fmt.Errorf("%w: geth offlinev", err)
	}
	if err := WaitForBeaconNode(ctx); err != nil {
		return fmt.Errorf("%w: beacon node offline", err)
	}
	if err := WaitForBeaconNodeFollower(ctx); err != nil {
		return fmt.Errorf("%w: beacon node follower offline", err)
	}
	if err := WaitForValidator(ctx); err != nil {
		return fmt.Errorf("%w: validator is offline", err)
	}
	return nil
}

var Env *TestEnvironment

func InitE2ETest() {
	if err := RestartDevnet(); err != nil {
		log.Fatalf("unable to restart devnet: %v", err)
	}
	if err := WaitForServices(context.Background()); err != nil {
		log.Fatal(err)
	}

	Env = newTestEnvironment()
}

func WaitForShardingFork() {
	ctx := context.Background()

	config := Env.GethChainConfig
	eip4844ForkBlock := config.ShardingForkBlock.Uint64()

	stallTimeout := 1 * time.Minute

	log.Printf("waiting for sharding fork block...")
	var lastBn uint64
	var lastUpdate time.Time
	for {
		bn, err := Env.EthClient.BlockNumber(ctx)
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
	req := &ethpbv1.BlockRequest{BlockId: []byte("head")}
	for {
		header, err := Env.BeaconChainClient.GetBlockHeader(ctx, req)
		if err != nil {
			return fmt.Errorf("unable to retrieve block header: %v", err)
		}
		headSlot := header.Data.Header.Message.Slot
		if headSlot >= slot {
			break
		}
		time.Sleep(time.Second * time.Duration(Env.BeaconChainConfig.SecondsPerSlot))
	}
	return nil
}

func WaitForEip4844ForkEpoch() {
	log.Printf("waiting for eip4844 fork epoch...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	config := Env.BeaconChainConfig
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
	GethChainConfig   *params.ChainConfig
	BeaconChainConfig *BeaconChainConfig
	EthClient         *ethclient.Client
	BeaconChainClient beaconservice.BeaconChainClient
}

func newTestEnvironment() *TestEnvironment {
	ctx := context.Background()
	eclient, err := ethclient.DialContext(ctx, shared.GethRPC)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	beaconGRPCConn, err := grpc.DialContext(ctx, shared.BeaconRPC, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial beacon grpc", err)
	}

	return &TestEnvironment{
		GethChainConfig:   ReadGethChainConfig(),
		BeaconChainConfig: ReadBeaconChainConfig(),
		EthClient:         eclient,
		BeaconChainClient: beaconservice.NewBeaconChainClient(beaconGRPCConn),
	}
}
