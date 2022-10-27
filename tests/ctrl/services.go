package ctrl

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/ethclient"
	beaconservice "github.com/prysmaticlabs/prysm/v3/proto/eth/service"
	"google.golang.org/grpc"
)

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Started() <-chan struct{}
}

func NewBeaconNode() Service {
	url := fmt.Sprintf("http://%s/eth/v1/beacon/genesis", shared.BeaconGatewayGRPC)
	return newDockerService("beacon-node", url)
}

func NewValidatorNode() Service {
	return newDockerService("validator-node", shared.ValidatorRPC)
}

func GetBeaconNodeClient(ctx context.Context) (beaconservice.BeaconChainClient, error) {
	// TODO: cache conns for reuse
	conn, err := grpc.DialContext(ctx, shared.BeaconRPC, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("%w: failed to dial beacon grpc", err)
	}
	return beaconservice.NewBeaconChainClient(conn), nil
}

func NewBeaconNodeFollower() Service {
	url := fmt.Sprintf("http://%s/eth/v1/beacon/genesis", shared.BeaconGatewayFollowerGRPC)
	return newDockerService("beacon-node-follower", url)
}

func GetBeaconNodeFollowerClient(ctx context.Context) (beaconservice.BeaconChainClient, error) {
	// TODO: cache conns for reuse
	conn, err := grpc.DialContext(ctx, shared.BeaconFollowerRPC, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("%w: failed to dial beacon follower grpc", err)
	}
	return beaconservice.NewBeaconChainClient(conn), nil
}

func NewGethNode() Service {
	return newDockerService("execution-node", shared.GethRPC)
}

func NewGethNode2() Service {
	return newDockerService("execution-node-2", shared.GethRPC)
}

func GetExecutionClient(ctx context.Context) (*ethclient.Client, error) {
	client, err := ethclient.DialContext(ctx, shared.GethRPC)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to connect to the Ethereum client", err)
	}
	return client, nil
}

type dockerService struct {
	started   chan struct{}
	svcname   string
	statusURL string
}

func (s *dockerService) Start(ctx context.Context) error {
	if err := StartServices(s.svcname); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", s.statusURL, nil)
	if err != nil {
		return err
	}
	// loop until the status request returns successfully
	for {
		if _, err := http.DefaultClient.Do(req); err == nil {
			close(s.started)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func (s *dockerService) Stop(ctx context.Context) error {
	return StopService(s.svcname)
}

func (s *dockerService) Started() <-chan struct{} {
	return s.started
}

func ServiceReady(ctx context.Context, svc Service) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-svc.Started():
			return nil
		}
	}
}

func newDockerService(svcname string, statusURL string) Service {
	return &dockerService{
		started:   make(chan struct{}),
		svcname:   svcname,
		statusURL: statusURL,
	}
}
