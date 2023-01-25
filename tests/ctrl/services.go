package ctrl

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
)

type Service interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Started() <-chan struct{}
}

func NewBeaconNode(clientName string) Service {
	url := fmt.Sprintf("http://%s/eth/v1/beacon/genesis", shared.BeaconAPI)
	return newDockerService(fmt.Sprintf("%s-beacon-node", clientName), url)
}

func NewValidatorNode(clientName string) Service {
	return newDockerService(fmt.Sprintf("%s-validator-node", clientName), "")
}

func NewBeaconNodeFollower(clientName string) Service {
	url := fmt.Sprintf("http://%s/eth/v1/beacon/genesis", shared.BeaconFollowerAPI)
	return newDockerService(fmt.Sprintf("%s-beacon-node-follower", clientName), url)
}

func GetBeaconNodeClient(ctx context.Context) (*beacon.Client, error) {
	client, err := beacon.NewClient(shared.BeaconAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create beacon API client", err)
	}
	return client, nil
}

func GetBeaconNodeFollowerClient(ctx context.Context) (*beacon.Client, error) {
	client, err := beacon.NewClient(shared.BeaconFollowerAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create beacon follower API client", err)
	}
	return client, nil
}

func NewGethNode() Service {
	return newDockerService("geth-1", shared.GethRPC)
}

func NewGethNode2() Service {
	return newDockerService("geth-2", shared.GethRPC)
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
	if s.statusURL == "" {
		return nil
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
			log.Printf("%s: waiting for a successful status check at %s", s.svcname, s.statusURL)
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
