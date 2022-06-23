package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Inphi/eip4844-interop/shared"

	ma "github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p"
	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	types "github.com/prysmaticlabs/prysm/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/proto/prysm/v1alpha1"
)

func main() {
	start := flag.Uint64("start", 0, "Start slot to download blobs from")
	count := flag.Uint64("count", 1, "Number of slots to download blobs from (default: 1)")
	addr := flag.String("addr", "", "P2P address to connect to")
	flag.Parse()

	if *start == 0 {
		panic("start parameter must be greater than 0")
	}
	if *addr == "" {
		panic("missing addr parameter")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := &ethpb.BlobsSidecarsByRangeRequest{
		StartSlot: types.Slot(*start),
		Count:     *count,
	}

	h, err := libp2p.New()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = h.Close()
	}()

	multiaddr, err := getMultiaddr(ctx, h, *addr)
	if err != nil {
		panic(err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiaddr)
	if err != nil {
		panic(err)
	}

	err = h.Connect(ctx, *addrInfo)
	if err != nil {
		panic(err)
	}

	sidecars, err := sendBlobsSidecarsByRangeRequest(ctx, h, encoder.SszNetworkEncoder{}, addrInfo.ID, req)
	if err != nil {
		panic(err)
	}

	anyBlobs := false
	for _, sidecar := range sidecars {
		if sidecar.Blobs == nil || len(sidecar.Blobs) == 0 {
			continue
		}
		anyBlobs = true
		data := shared.DecodeBlobs(sidecar)
		i := len(data) - 1
		for ; i >= 0; i-- {
			if data[i] != 0x00 {
				break
			}
		}
		data = data[:i+1]
		_, _ = os.Stdout.Write(data)

		// stop after the first sidecar with blobs:
		break
	}

	if !anyBlobs {
		panic(fmt.Sprintf("No blobs found in requested slots, sidecar count: %d", len(sidecars)))
	}
}

func getMultiaddr(ctx context.Context, h host.Host, addr string) (ma.Multiaddr, error) {
	multiaddr, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	_, id := peer.SplitAddr(multiaddr)
	if id != "" {
		return multiaddr, nil
	}
	// peer ID wasn't provided, look it up
	id, err = retrievePeerID(ctx, h, addr)
	if err != nil {
		return nil, err
	}
	return ma.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", addr, string(id)))
}

// Helper for retrieving the peer ID from a security error... obviously don't use this in production!
// See https://github.com/libp2p/go-libp2p-noise/blob/v0.3.0/handshake.go#L250
func retrievePeerID(ctx context.Context, h host.Host, addr string) (peer.ID, error) {
	incorrectPeerID := "16Uiu2HAmSifdT5QutTsaET8xqjWAMPp4obrQv7LN79f2RMmBe3nY"
	addrInfo, err := peer.AddrInfoFromString(fmt.Sprintf("%s/p2p/%s", addr, incorrectPeerID))
	if err != nil {
		return "", err
	}
	err = h.Connect(ctx, *addrInfo)
	if err == nil {
		return "", errors.New("unexpected successful connection")
	}
	if strings.Contains(err.Error(), "but remote key matches") {
		split := strings.Split(err.Error(), " ")
		return peer.ID(split[len(split)-1]), nil
	}
	return "", err
}

func sendBlobsSidecarsByRangeRequest(ctx context.Context, h host.Host, encoding encoder.NetworkEncoding, pid peer.ID, req *ethpb.BlobsSidecarsByRangeRequest) ([]*ethpb.BlobsSidecar, error) {
	topic := fmt.Sprintf("%s%s", p2p.RPCBlobsSidecarsByRangeTopicV1, encoding.ProtocolSuffix())

	stream, err := h.NewStream(ctx, pid, protocol.ID(topic))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = stream.Close()
	}()

	if _, err := encoding.EncodeWithMaxLength(stream, req); err != nil {
		_ = stream.Reset()
		return nil, err
	}

	if err := stream.CloseWrite(); err != nil {
		_ = stream.Reset()
		return nil, err
	}

	var blobsSidecars []*ethpb.BlobsSidecar
	for {
		blobs, err := readChunkedBlobsSidecar(stream, encoding)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		blobsSidecars = append(blobsSidecars, blobs)
	}
	return blobsSidecars, nil
}

func readChunkedBlobsSidecar(stream libp2pcore.Stream, encoding encoder.NetworkEncoding) (*ethpb.BlobsSidecar, error) {
	code, errMsg, err := sync.ReadStatusCode(stream, encoding)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, errors.New(errMsg)
	}
	sidecar := new(ethpb.BlobsSidecar)
	err = encoding.DecodeWithMaxLength(stream, sidecar)
	return sidecar, err
}
