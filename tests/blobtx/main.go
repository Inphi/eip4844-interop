package main

import (
	"bytes"
	"context"
	"fmt"
	beaconservice "github.com/prysmaticlabs/prysm/v3/proto/eth/service"
	"log"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/ctrl"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
	ethpbv1 "github.com/prysmaticlabs/prysm/v3/proto/eth/v1"
	ethpbv2 "github.com/prysmaticlabs/prysm/v3/proto/eth/v2"
)

func GetBlobs() types.Blobs {
	// dummy data for the test
	dat, err := os.ReadFile("./eth.png")
	if err != nil {
		log.Fatalf("error reading blobs file: %v", err)
	}
	return shared.EncodeBlobs(dat)
}

// 1. Uploads blobs
// 2. Downloads blobs
// 3. Asserts that downloaded blobs match the upload
// 4. Asserts execution and beacon block attributes
func main() {
	ctrl.InitE2ETest()
	ctrl.WaitForShardingFork()
	ctrl.WaitForEip4844ForkEpoch()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	blobs := GetBlobs()

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := GetHeadSlot(ctx)

	UploadBlobs(ctx, blobs)
	WaitForNextSlot(ctx)
	slot := FindBlobSlot(ctx, startSlot)

	log.Printf("checking blob from beacon node")
	downloadedData := DownloadBlobs(ctx, slot, ctrl.Env.BeaconChainClient)
	downloadedBlobs := shared.EncodeBlobs(downloadedData)
	AssertBlobsEquals(blobs, downloadedBlobs)

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 2 * time.Duration(ctrl.Env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync
	downloadedData = DownloadBlobs(ctx, slot, ctrl.Env.BeaconChainFollowerClient)
	downloadedBlobs = shared.EncodeBlobs(downloadedData)
	AssertBlobsEquals(blobs, downloadedBlobs)
}

func AssertBlobsEquals(a, b types.Blobs) {
	// redundant for nice for debugging
	if len(a) != len(b) {
		log.Fatalf("data length mismatch (%d != %d)", len(a), len(b))
	}
	for i := range a {
		for j := 0; j < params.FieldElementsPerBlob; j++ {
			if !bytes.Equal(a[i][j][:], b[i][j][:]) {
				log.Fatal("blobs data mismatch")
			}
		}
	}
}

func WaitForNextSlot(ctx context.Context) {
	if err := ctrl.WaitForSlot(ctx, GetHeadSlot(ctx).Add(1)); err != nil {
		log.Fatalf("error waiting for next slot: %v", err)
	}
}

func GetHeadSlot(ctx context.Context) consensustypes.Slot {
	req := &ethpbv1.BlockRequest{BlockId: []byte("head")}
	header, err := ctrl.Env.BeaconChainClient.GetBlockHeader(ctx, req)
	if err != nil {
		log.Fatalf("unable to get beacon chain head: %v", err)
	}
	return header.Data.Header.Message.Slot
}

func UploadBlobs(ctx context.Context, blobs types.Blobs) {
	chainId := big.NewInt(1)
	signer := types.NewDankSigner(chainId)

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := ctrl.Env.EthClient.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()

	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	txData := types.SignedBlobTx{
		Message: types.BlobTxMessage{
			ChainID:             view.Uint256View(*uint256.NewInt(chainId.Uint64())),
			Nonce:               view.Uint64View(nonce),
			Gas:                 210000,
			GasFeeCap:           view.Uint256View(*uint256.NewInt(5000000000)),
			GasTipCap:           view.Uint256View(*uint256.NewInt(5000000000)),
			Value:               view.Uint256View(*uint256.NewInt(12345678)),
			To:                  types.AddressOptionalSSZ{Address: (*types.AddressSSZ)(&to)},
			BlobVersionedHashes: versionedHashes,
		},
	}

	wrapData := types.BlobTxWrapData{
		BlobKzgs:           commitments,
		Blobs:              blobs,
		KzgAggregatedProof: aggregatedProof,
	}
	tx := types.NewTx(&txData, types.WithTxWrapData(&wrapData))
	tx, err = types.SignTx(tx, signer, key)
	if err != nil {
		log.Fatalf("Error signing tx: %v", err)
	}
	err = ctrl.Env.EthClient.SendTransaction(ctx, tx)
	if err != nil {
		log.Fatalf("Error sending tx: %v", err)
	}
	log.Printf("Transaction submitted. hash=%v", tx.Hash())

	log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())
	if _, err := shared.WaitForReceipt(ctx, ctrl.Env.EthClient, tx.Hash()); err != nil {
		log.Fatalf("Error waiting for transaction receipt %v: %v", tx.Hash(), err)
	}
}

func FindBlobSlot(ctx context.Context, startSlot consensustypes.Slot) consensustypes.Slot {
	slot := startSlot
	endSlot := GetHeadSlot(ctx)
	for {
		if slot == endSlot {
			log.Fatalf("Unable to find beacon block containing blobs")
		}

		blockID := fmt.Sprintf("%d", uint64(slot))
		req := &ethpbv2.BlockRequestV2{BlockId: []byte(blockID)}
		block, err := ctrl.Env.BeaconChainClient.GetBlockV2(ctx, req)
		if err != nil {
			log.Fatalf("beaconchainclient.GetBlock: %v", err)
		}
		eip4844, ok := block.Data.Message.(*ethpbv2.SignedBeaconBlockContainer_Eip4844Block)
		if ok {
			if len(eip4844.Eip4844Block.Body.BlobKzgs) != 0 {
				return eip4844.Eip4844Block.Slot
			}
		}

		slot = slot.Add(1)
	}
}

func DownloadBlobs(ctx context.Context, startSlot consensustypes.Slot, client beaconservice.BeaconChainClient) []byte {
	log.Print("downloading blobs...")

	req := ethpbv1.BlobsRequest{BlockId: []byte(strconv.FormatUint(uint64(startSlot), 10))}
	sidecar, err := client.GetBlobsSidecar(ctx, &req)
	if err != nil {
		log.Fatalf("failed to send blobs sidecar request: %v", err)
	}

	blobsBuffer := new(bytes.Buffer)
	for _, blob := range sidecar.Blobs {
		data := shared.DecodeBlob(blob.Data)
		_, _ = blobsBuffer.Write(data)
	}

	return blobsBuffer.Bytes()
}
