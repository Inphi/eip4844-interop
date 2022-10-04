package main

import (
	"bytes"
	"context"
	"fmt"
	beaconservice "github.com/prysmaticlabs/prysm/v3/proto/eth/service"
	"log"
	"math/big"
	"sort"
	"strconv"
	"sync"
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

func GetBlob() types.Blobs {
	// dummy data for the test
	return shared.EncodeBlobs([]byte("EKANS"))
}

// 1. Uploads *multiple* blobs
// 2. Downloads blobs
// 3. Asserts that downloaded blobs match the upload
// 4. Asserts execution and beacon block attributes
func main() {
	ctrl.InitE2ETest()
	ctrl.WaitForShardingFork()
	ctrl.WaitForEip4844ForkEpoch()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	blobsData := make([]types.Blobs, 20)
	for i := range blobsData {
		blobsData[i] = GetBlob()
	}

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := GetHeadSlot(ctx)

	// Send multiple transactions at the same time to induce non-zero excess_blobs
	UploadBlobsAndCheckBlockHeader(ctx, blobsData)

	WaitForNextSlot(ctx)
	WaitForNextSlot(ctx)

	blocks := FindBlocksWithBlobs(ctx, startSlot)

	log.Printf("checking blob from beacon node")
	var downloadedData []byte
	for _, b := range blocks {
		data := DownloadBlobs(ctx, b.Slot, ctrl.Env.BeaconChainClient)
		downloadedData = append(downloadedData, data...)
	}

	flatBlobs := FlattenBlobs(blobsData)

	if !bytes.Equal(flatBlobs, downloadedData) {
		log.Fatalf("mismatch %d %v", len(flatBlobs), len(downloadedData))
	}

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 2 * time.Duration(ctrl.Env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync

	downloadedData = nil
	for _, b := range blocks {
		data := DownloadBlobs(ctx, b.Slot, ctrl.Env.BeaconChainFollowerClient)
		downloadedData = append(downloadedData, data...)
	}
	if !bytes.Equal(flatBlobs, downloadedData) {
		log.Fatalf("mismatch %d %v", len(flatBlobs), len(downloadedData))
	}
}

func FlattenBlobs(blobsData []types.Blobs) []byte {
	var out []byte
	for _, blobs := range blobsData {
		for _, blob := range blobs {
			rawBlob := make([][]byte, len(blob))
			for i := range blob {
				rawBlob[i] = blob[i][:]
			}
			decoded := shared.DecodeBlobs(rawBlob)
			out = append(out, decoded...)
		}
	}
	return out
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

func UploadBlobsAndCheckBlockHeader(ctx context.Context, blobsData []types.Blobs) {
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

	var txs []*types.Transaction
	for i := range blobsData {
		blobs := blobsData[i]
		commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()
		to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
		txData := types.SignedBlobTx{
			Message: types.BlobTxMessage{
				ChainID:             view.Uint256View(*uint256.NewInt(chainId.Uint64())),
				Nonce:               view.Uint64View(nonce + uint64(i)),
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
		txs = append(txs, tx)
	}

	receipts := make(chan *types.Receipt, len(txs))
	var wg sync.WaitGroup
	wg.Add(len(txs))
	for _, tx := range txs {
		tx := tx
		go func() {
			defer wg.Done()
			err := ctrl.Env.EthClient.SendTransaction(ctx, tx)
			if err != nil {
				log.Fatalf("Error sending tx: %v", err)
			}

			log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())

			receipt, err := shared.WaitForReceipt(ctx, ctrl.Env.EthClient, tx.Hash())
			if err != nil {
				log.Fatalf("Error waiting for transaction receipt %v: %v", tx.Hash(), err)
			}
			receipts <- receipt
		}()
	}
	wg.Wait()
	close(receipts)

	log.Printf("checking mined blocks...")
	blockNumbers := make(map[uint64]bool)
	var blocks []*types.Block
	for receipt := range receipts {
		blocknum := receipt.BlockNumber.Uint64()
		if _, ok := blockNumbers[blocknum]; !ok {
			blockHash := receipt.BlockHash.Hex()
			block, err := ctrl.Env.EthClient.BlockByHash(ctx, common.HexToHash(blockHash))
			if err != nil {
				log.Fatalf("Error getting block: %v", err)
			}
			excessBlobs := block.ExcessBlobs()
			if excessBlobs == nil {
				log.Fatalf("nil excess_blobs in block header. block_hash=%v", blockHash)
			}
			blockNumbers[blocknum] = true
			blocks = append(blocks, block)
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number().Uint64() < blocks[j].Number().Uint64()
	})

	var prevExcessBlobs int
	parentBlock, err := ctrl.Env.EthClient.BlockByHash(ctx, blocks[0].ParentHash())
	if err != nil {
		log.Fatalf("Error getting block: %v", err)
	}
	if e := parentBlock.ExcessBlobs(); e != nil {
		prevExcessBlobs = int(*e)
	}

	for _, block := range blocks {
		// Assuming each transaction contains a single blob
		adjusted := len(block.Transactions()) + prevExcessBlobs
		var expected int
		if adjusted > params.TargetBlobsPerBlock {
			expected = adjusted - params.TargetBlobsPerBlock
		}
		if expected != int(*block.ExcessBlobs()) {
			log.Fatal("unexpected excess_blobs field in header. expected %v. got %v", expected, *block.ExcessBlobs())
		}
		prevExcessBlobs = expected
	}
}

func FindBlocksWithBlobs(ctx context.Context, startSlot consensustypes.Slot) []*ethpbv2.BeaconBlockEip4844 {
	slot := startSlot
	endSlot := GetHeadSlot(ctx)

	var blocks []*ethpbv2.BeaconBlockEip4844
	for {
		if slot == endSlot {
			break
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
				blocks = append(blocks, eip4844.Eip4844Block)
			}
		}

		slot = slot.Add(1)
	}

	if len(blocks) == 0 {
		log.Fatalf("Unable to find beacon block containing blobs")
	}
	return blocks
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
