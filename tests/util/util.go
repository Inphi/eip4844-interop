package util

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/pkg/errors"
	ssz "github.com/prysmaticlabs/fastssz"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
	"github.com/prysmaticlabs/prysm/v3/beacon-chain/p2p/encoder"
	"github.com/prysmaticlabs/prysm/v3/consensus-types/blocks"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	"github.com/prysmaticlabs/prysm/v3/encoding/bytesutil"
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
	BlobKzgCommitments [][]byte
}

type Message struct {
	Slot consensustypes.Slot
	Body Body
}

type Data struct {
	Message Message
}

type Block struct {
	Data Data
}

var (
	// Hardcoded versions for now
	GenesisVersion   = [4]byte{0x20, 0x00, 0x00, 0x89}
	AltairVersion    = [4]byte{0x20, 0x00, 0x00, 0x90}
	BellatrixVersion = [4]byte{0x20, 0x00, 0x00, 0x91}
	CapellaVersion   = [4]byte{0x20, 0x00, 0x00, 0x92}
	EIP4844Version   = [4]byte{0x20, 0x00, 0x00, 0x93}
)

func GetBlock(ctx context.Context, client *beacon.Client, blockId beacon.StateOrBlockId) (*Block, error) {
	sb, err := client.GetState(ctx, blockId)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get head state")
	}
	version, err := extractVersionFromState(sb)
	if err != nil {
		return nil, err
	}

	var m ssz.Unmarshaler
	switch version {
	case GenesisVersion:
		m = &ethpb.SignedBeaconBlock{}
	case AltairVersion:
		m = &ethpb.SignedBeaconBlockAltair{}
	case BellatrixVersion:
		m = &ethpb.SignedBeaconBlockBellatrix{}
	case CapellaVersion:
		m = &ethpb.SignedBeaconBlockCapella{}
	case EIP4844Version:
		m = &ethpb.SignedBeaconBlock4844{}
	default:
		return nil, fmt.Errorf("unable to initialize beacon block for fork version=%x at blockId=%s", version, blockId)
	}

	marshaled, err := client.GetBlock(ctx, blockId)
	if err != nil {
		log.Fatalf("unable to get beacon chain block: %v", err)
	}
	err = m.UnmarshalSSZ(marshaled)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal SignedBeaconBlock in UnmarshalSSZ")
	}
	blk, err := blocks.NewSignedBeaconBlock(m)
	if err != nil {
		return nil, err
	}

	kzgs, _ := blk.Block().Body().BlobKzgCommitments()

	var block Block
	block.Data.Message = Message{
		Slot: blk.Block().Slot(),
		Body: Body{
			BlobKzgCommitments: kzgs,
		},
	}
	return &block, nil
}

func GetHeadSlot(ctx context.Context, client *beacon.Client) consensustypes.Slot {
	block, err := GetBlock(ctx, client, "head")
	if err != nil {
		log.Fatalf("GetBlock error: %v", err)
	}
	return block.Data.Message.Slot
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

		if len(block.Data.Message.Body.BlobKzgCommitments) != 0 {
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

// extractVersionFromState reads the beacon state version from the ssz in-situ
func extractVersionFromState(state []byte) ([4]byte, error) {
	size := 4 // field size
	// Using ssz offset in BeaconState:
	// 52 = 8 (genesis_time) + 32 (genesis_validators_root) + 8 (slot) + 4 (previous_version)
	offset := 52
	if len(state) < offset+size {
		return [4]byte{}, errors.New("invalid state. index out of range")
	}
	val := state[offset : offset+size]
	return bytesutil.ToBytes4(val), nil
}
