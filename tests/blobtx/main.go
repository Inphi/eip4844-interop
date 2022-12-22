package main

import (
	"context"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/ctrl"
	"github.com/Inphi/eip4844-interop/tests/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
)

func GetBlobs() types.Blobs {
	// dummy data for the test
	return shared.EncodeBlobs([]byte("EKANS"))
}

// 1. Uploads blobs
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

	blobs := GetBlobs()

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := util.GetHeadSlot(ctx, beaconClient)

	chainID := env.GethChainConfig.ChainID
	UploadBlobs(ctx, ethClient, chainID, blobs)
	util.WaitForNextSlots(ctx, beaconClient, 1)
	slot := util.FindBlobSlot(ctx, beaconClient, startSlot)

	multiaddr, err := shared.GetBeaconMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}
	followerMultiaddr, err := shared.GetBeaconMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}

	log.Printf("checking blob from beacon node")
	downloadedData := util.DownloadBlobs(ctx, slot, 1, multiaddr)
	downloadedBlobs := shared.EncodeBlobs(downloadedData)
	util.AssertBlobsEquals(blobs, downloadedBlobs)

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 10 * time.Duration(env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync
	downloadedData = util.DownloadBlobs(ctx, slot, 1, followerMultiaddr)
	downloadedBlobs = shared.EncodeBlobs(downloadedData)
	util.AssertBlobsEquals(blobs, downloadedBlobs)
}

func UploadBlobs(ctx context.Context, client *ethclient.Client, chainID *big.Int, blobs types.Blobs) {
	signer := types.NewDankSigner(chainID)

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()

	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	txData := types.SignedBlobTx{
		Message: types.BlobTxMessage{
			ChainID:             view.Uint256View(*uint256.NewInt(chainID.Uint64())),
			Nonce:               view.Uint64View(nonce),
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
	err = client.SendTransaction(ctx, tx)
	if err != nil {
		log.Fatalf("Error sending tx: %v", err)
	}
	log.Printf("Transaction submitted. hash=%v", tx.Hash())

	log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())
	if _, err := shared.WaitForReceipt(ctx, client, tx.Hash()); err != nil {
		log.Fatalf("Error waiting for transaction receipt %v: %v", tx.Hash(), err)
	}
}
