package main

import (
	"bytes"
	"context"
	"log"
	"math/big"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/ctrl"
	"github.com/Inphi/eip4844-interop/tests/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
	"github.com/prysmaticlabs/prysm/v3/api/client/beacon"
	consensustypes "github.com/prysmaticlabs/prysm/v3/consensus-types/primitives"
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
	clientName := "prysm"
	if len(os.Args) > 1 {
		clientName = os.Args[1]
	}
	ctrl.InitE2ETest(clientName)
	ctrl.WaitForShardingFork()
	ctrl.WaitForEip4844ForkEpoch()
	env := ctrl.GetEnv()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	ethClient, err := ctrl.GetExecutionClient(ctx)
	if err != nil {
		log.Fatalf("unable to get execution client: %v", err)
	}
	beaconClient, err := ctrl.GetBeaconNodeClient(ctx)
	if err != nil {
		log.Fatalf("unable to get beacon client: %v", err)
	}

	blobsData := make([]types.Blobs, 20)
	for i := range blobsData {
		blobsData[i] = GetBlob()
	}

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := util.GetHeadSlot(ctx, beaconClient)

	// Send multiple transactions at the same time to induce non-zero excess_blobs
	chainID := env.GethChainConfig.ChainID
	UploadBlobsAndCheckBlockHeader(ctx, ethClient, chainID, blobsData)

	util.WaitForNextSlots(ctx, beaconClient, 1)
	util.WaitForNextSlots(ctx, beaconClient, 1)

	log.Print("blobs uploaded. finding blocks with blobs")

	blocks := FindBlocksWithBlobs(ctx, beaconClient, startSlot)

	log.Printf("checking blob from beacon node")
	ma, err := shared.GetBeaconMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}
	var downloadedData []byte
	for _, b := range blocks {
		data := util.DownloadBlobs(ctx, b.Data.Message.Slot, 1, ma)
		downloadedData = append(downloadedData, data...)
	}

	flatBlobs := FlattenBlobs(blobsData)

	if !bytes.Equal(flatBlobs, downloadedData) {
		log.Fatalf("mismatch %d %v", len(flatBlobs), len(downloadedData))
	}

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 2 * time.Duration(env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync

	downloadedData = nil
	maFollower, err := shared.GetBeaconFollowerMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}
	for _, b := range blocks {
		data := util.DownloadBlobs(ctx, b.Data.Message.Slot, 1, maFollower)
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
			decoded := shared.DecodeBlob(rawBlob)
			out = append(out, decoded...)
		}
	}
	return out
}

func UploadBlobsAndCheckBlockHeader(ctx context.Context, client *ethclient.Client, chainId *big.Int, blobsData []types.Blobs) {
	signer := types.NewDankSigner(chainId)

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
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
				MaxFeePerDataGas:    view.Uint256View(*uint256.NewInt(3000000000)), // needs to be at least the min fee
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
			err := client.SendTransaction(ctx, tx)
			if err != nil {
				log.Fatalf("Error sending tx: %v", err)
			}

			log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())

			receipt, err := shared.WaitForReceipt(ctx, client, tx.Hash())
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
			block, err := client.BlockByHash(ctx, common.HexToHash(blockHash))
			if err != nil {
				log.Fatalf("Error getting block: %v", err)
			}
			excessDataGas := block.ExcessDataGas()
			if excessDataGas == nil {
				log.Fatalf("nil excess_blobs in block header. block_hash=%v", blockHash)
			}
			blockNumbers[blocknum] = true
			blocks = append(blocks, block)
		}
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Number().Uint64() < blocks[j].Number().Uint64()
	})

	prevExcessDataGas := new(big.Int)
	parentBlock, err := client.BlockByHash(ctx, blocks[0].ParentHash())
	if err != nil {
		log.Fatalf("Error getting block: %v", err)
	}
	if e := parentBlock.ExcessDataGas(); e != nil {
		prevExcessDataGas.Set(e)
	}

	for _, block := range blocks {
		// Assuming each transaction contains a single blob
		expected := misc.CalcExcessDataGas(prevExcessDataGas, len(block.Transactions()))
		if expected.Cmp(block.ExcessDataGas()) != 0 {
			log.Fatalf("unexpected excess_data_gas field in header. expected %v. got %v", expected, block.ExcessDataGas())
		}
		prevExcessDataGas = expected
	}
}

func FindBlocksWithBlobs(ctx context.Context, client *beacon.Client, startSlot consensustypes.Slot) []*util.Block {
	slot := startSlot
	endSlot := util.GetHeadSlot(ctx, client)

	var blocks []*util.Block
	for {
		if slot == endSlot {
			break
		}

		block, err := util.GetBlock(ctx, client, beacon.IdFromSlot(slot))
		if err != nil {
			log.Fatalf("Failed to GetBlock: %v", err)
		}

		if len(block.Data.Message.Body.BlobKzgCommitments) != 0 {
			blocks = append(blocks, block)
		}

		slot = slot.Add(1)
	}

	if len(blocks) == 0 {
		log.Fatalf("Unable to find beacon block containing blobs")
	}
	return blocks
}
