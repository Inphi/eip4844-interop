package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/libp2p/go-libp2p"
	libp2pcore "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/sync"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	ethpb "github.com/prysmaticlabs/prysm/v3/proto/prysm/v1alpha1"
)

func init() {
	encoder.MaxChunkSize = 10 << 20
}

func WaitForSlot(ctx context.Context, client *beacon.Client, slot consensustypes.Slot) error {
	for {
		headSlot := GetHeadSlot(ctx, client)
		if headSlot >= slot {
			break
		}
		time.Sleep(time.Second * 1)
	}
	return nil
}

func WaitForNextSlots(ctx context.Context, client *beacon.Client, slots consensustypes.Slot) {
	if err := WaitForSlot(ctx, client, GetHeadSlot(ctx, client).AddSlot(slots)); err != nil {
		log.Fatalf("error waiting for next slot: %v", err)
	}
}

type Body struct {
	BlobKzgs []string `json:"blob_kzgs"`
}

type Message struct {
	Slot string
	Body Body
}

type Data struct {
	Message Message
}

type Block struct {
	Data Data
}

func GetBlock(ctx context.Context, client *beacon.Client, blockId beacon.StateOrBlockId) (Block, error) {
	blockJSON, err := client.GetBlockJSON(ctx, blockId)
	if err != nil {
		log.Fatalf("unable to get beacon chain block: %v", err)
	}
	var block Block
	err = json.Unmarshal(blockJSON, &block)
	if err != nil {
		log.Fatalf("unable to unmarshall beacon chain block JSON: %v", err)
	}

	return block, nil
}

func GetHeadSlot(ctx context.Context, client *beacon.Client) consensustypes.Slot {
	block, _ := GetBlock(ctx, client, "head")
	slot, err := strconv.ParseUint(block.Data.Message.Slot, 10, 64)
	if err != nil {
		log.Fatalf("unable to parse beacon chain head slot: %v", err)
	}
	return (consensustypes.Slot)(slot)
}

// FindBlobSlot returns the first slot containing a blob since startSlot
// Panics if no such slot could be found
func FindBlobSlot(ctx context.Context, client *beacon.Client, startSlot consensustypes.Slot) consensustypes.Slot {
	slot := startSlot
	endSlot := GetHeadSlot(ctx, client)
	for {
		if slot == endSlot {
			log.Fatalf("Unable to find beacon block containing blobs")
		}

		block, err := GetBlock(ctx, client, beacon.IdFromSlot(slot))
		if err != nil {
			log.Fatalf("beaconchainclient.GetBlock: %v", err)
		}

		if len(block.Data.Message.Body.BlobKzgs) != 0 {
			return slot
		}

		slot = slot.Add(1)
	}
}

func AssertBlobsEquals(a, b types.Blobs) {
	if len(a) != len(b) {
		log.Fatalf("data length mismatch (%d != %d)", len(a), len(b))
	}
	for i, _ := range a {
		for j := 0; j < params.FieldElementsPerBlob; j++ {
			if !bytes.Equal(a[i][j][:], b[i][j][:]) {
				log.Fatal("blobs data mismatch")
			}
		}
	}
}

func SendBlobsSidecarsByRangeRequest(ctx context.Context, h host.Host, encoding encoder.NetworkEncoding, pid peer.ID, req *ethpb.BlobsSidecarsByRangeRequest) ([]*ethpb.BlobsSidecar, error) {
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

// Using p2p RPC
func DownloadBlobs(ctx context.Context, startSlot consensustypes.Slot, count uint64, beaconMA string) []byte {
	log.Print("downloading blobs...")

	req := &ethpb.BlobsSidecarsByRangeRequest{
		StartSlot: startSlot,
		Count:     count,
	}

	h, err := libp2p.New()
	if err != nil {
		log.Fatalf("failed to create libp2p context: %v", err)
	}
	defer func() {
		_ = h.Close()
	}()

	multiaddr, err := getMultiaddr(ctx, h, beaconMA)
	if err != nil {
		log.Fatalf("getMultiAddr: %v", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiaddr)
	if err != nil {
		log.Fatal(err)
	}

	err = h.Connect(ctx, *addrInfo)
	if err != nil {
		log.Fatalf("libp2p host connect: %v", err)
	}

	sidecars, err := SendBlobsSidecarsByRangeRequest(ctx, h, encoder.SszNetworkEncoder{}, addrInfo.ID, req)
	if err != nil {
		log.Fatalf("failed to send blobs p2p request: %v", err)
	}

	anyBlobs := false
	blobsBuffer := new(bytes.Buffer)
	for _, sidecar := range sidecars {
		if sidecar.Blobs == nil || len(sidecar.Blobs) == 0 {
			continue
		}
		anyBlobs = true
		for _, blob := range sidecar.Blobs {
			data := shared.DecodeBlob(blob.Blob)
			_, _ = blobsBuffer.Write(data)
		}

		// stop after the first sidecar with blobs:
		break
	}

	if !anyBlobs {
		log.Fatalf("No blobs found in requested slots, sidecar count: %d", len(sidecars))
	}

	return blobsBuffer.Bytes()
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
