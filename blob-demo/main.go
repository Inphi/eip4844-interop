package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	ethpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
)

var (
	beaconNodeRPC = flag.String("beacon-node-rpc", "http://127.0.0.1:3500", "Beacon node RPC URL")
	startSlot     = flag.Int("start-slot", "0", "starting slot to fetch blobs")
	count         = flag.Int("count", "1", "number of blobs to fetch")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &ethpb.BlobsSidecarsByRangeRequest{
		StartSlot: *startSlot,
		Count:     *count,
	}

	svc, err := p2p.NewService(ctx, &p2p.Config{
		NoDiscovery:   true,
		DisableDiscv5: true,
	})
	if err != nil {
		log.Fatal("unable to create p2p service: %v", err)
	}

	addrStr := ReadPeerAddress(*beaconNodeRPC)

	addr, err := peer.AddrInfoFromString(addrStr)
	if err != nil {
		log.Fatal("Invalid multiaddr: %v", err)
	}

	err = svc.Connect(*addr)
	if err != nil {
		log.Fatal("failed to connect to beacon peer: %v", err)
	}
	defer func() {
		log.Warnf("disconnecting from peer: %v", svc.Disconnect(addr.ID))
	}()

	sidecars, err := SendBlobsSidecarsByRangeRequest(ctx, svc, addr.ID, req)
	if err != nil {
		log.Fatal(err)
	}
	//log.Infof("Got %d sidecars: %+v", len(sidecars), sidecars)
	log.Infof("Got %d sidecars", len(sidecars))
}

func SendBlobsSidecarsByRangeRequest(
	ctx context.Context, p2pProvider p2p.P2P, pid peer.ID, req *ethpb.BlobsSidecarsByRangeRequest) ([]*ethpb.BlobsSidecar, error) {
	stream, err := p2pProvider.Send(ctx, req, p2p.RPCBlobsSidecarsByRangeTopicV1, pid)
	if err != nil {
		return nil, fmt.Errorf("%w: unable to send BlobsSidecarsByRange request", err)
	}
	defer func() {
		_ = stream.Close()
	}()

	var blobsSidecars []*ethpb.BlobsSidecar
	for {
		blobs, err := ReadChunkedBlobsSidecar(stream, p2pProvider)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: ReadChunkedBlobsSidecar failed", err)
		}

		blobsSidecars = append(blobsSidecars, blobs)
	}
	return blobsSidecars, nil
}

func ReadChunkedBlobsSidecar(stream libp2pcore.Stream, p2p p2p.P2P) (*ethpb.BlobsSidecar, error) {
	code, errMsg, err := sync.ReadStatusCode(stream, p2p.Encoding())
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(errMsg)
	}
	sidecar := new(ethpb.BlobsSidecar)
	err = p2p.Encoding().DecodeWithMaxLength(stream, sidecar)
	return sidecar, err
}

func ReadPeerAddress(rpcURL string) string {
	url := fmt.Sprintf("%s/eth/v1/node/identity", rpcURL)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("peer RPC is unavailable: %v", err)
	}
	if resp.StatusCode != 200 {
		log.Fatalf("received bad status code: %d", resp.StatusCode)
	}

	type beaconNodeIdentityResponse struct {
		Data struct {
			P2PAddresses []string `json:"p2p_addresses"`
		} `json:"data"`
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("unable to read body: %v", err)
	}

	var beaconResponse beaconNodeIdentityResponse
	if err := json.Unmarshal(body, &beaconResponse); err != nil {
		log.Fatalf("failed to read/decode beacon ID response: %v", err)
	}
	if len(beaconResponse.Data.P2PAddresses) == 0 {
		log.Fatal("beacon node has no P2P addresses")
	}
	return beaconResponse.Data.P2PAddresses[0]
}
