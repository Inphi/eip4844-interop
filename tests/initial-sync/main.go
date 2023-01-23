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
	"golang.org/x/sync/errgroup"
)

func GetBlobs() types.Blobs {
	// dummy data for the test
	return shared.EncodeBlobs([]byte("EKANS"))
}

// Asserts blob syncing functionality during initial-sync
// 1. Start a single EL/CL node
// 2. Upload blobs
// 3. Wait for blobs to be available
// 4. Start follower EL/CL nodes
// 5. Download blobs from follower
// 6. Asserts that downloaded blobs match the upload
// 7. Asserts execution and beacon block attributes
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	ctrl.StopDevnet()

	clientName := "prysm"
	if len(os.Args) > 1 {
		clientName = os.Args[1]
	}
	env := ctrl.InitEnvForClient(clientName)

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return env.GethNode.Start(gctx)
	})
	g.Go(func() error {
		return env.BeaconNode.Start(gctx)
	})
	g.Go(func() error {
		return env.ValidatorNode.Start(gctx)
	})
	if err := g.Wait(); err != nil {
		log.Fatalf("failed to start services: %v", err)
	}
	ctrl.WaitForShardingFork()
	ctrl.WaitForEip4844ForkEpoch()

	ethClient, err := ctrl.GetExecutionClient(ctx)
	if err != nil {
		log.Fatalf("unable to get execution client: %v", err)
	}
	beaconClient, err := ctrl.GetBeaconNodeClient(ctx)
	if err != nil {
		log.Fatalf("unable to get beacon client: %v", err)
	}

	// Retrieve the current slot to being our blobs search on the beacon chain
	startSlot := util.GetHeadSlot(ctx, beaconClient)

	blobs := GetBlobs()
	UploadBlobs(ctx, ethClient, blobs)
	util.WaitForNextSlots(ctx, beaconClient, 1)
	blobSlot := util.FindBlobSlot(ctx, beaconClient, startSlot)

	// Wait a bit to induce substantial initial-sync in the beacon node follower
	util.WaitForNextSlots(ctx, beaconClient, 10)

	g.Go(func() error {
		return env.GethNode2.Start(ctx)
	})
	g.Go(func() error {
		return env.BeaconNodeFollower.Start(ctx)
	})
	if err := g.Wait(); err != nil {
		log.Fatalf("failed to start services: %v", err)
	}

	beaconNodeFollowerClient, err := ctrl.GetBeaconNodeFollowerClient(ctx)
	if err != nil {
		log.Fatalf("failed to get beacon node follower client: %v", err)
	}

	syncSlot := util.GetHeadSlot(ctx, beaconClient)
	if err := ctrl.WaitForSlotWithClient(ctx, beaconNodeFollowerClient, syncSlot); err != nil {
		log.Fatalf("unable to wait for beacon follower sync: %v", err)
	}

	multiaddr, err := shared.GetBeaconMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}
	followerMultiaddr, err := shared.GetBeaconFollowerMultiAddress()
	if err != nil {
		log.Fatalf("unable to get beacon multiaddr: %v", err)
	}

	log.Printf("checking blob from beacon node")
	downloadedData := util.DownloadBlobs(ctx, blobSlot, 1, multiaddr)
	downloadedBlobs := shared.EncodeBlobs(downloadedData)
	util.AssertBlobsEquals(blobs, downloadedBlobs)

	log.Printf("checking blob from beacon node follower")
	time.Sleep(time.Second * 2 * time.Duration(env.BeaconChainConfig.SecondsPerSlot)) // wait a bit for sync
	downloadedData = util.DownloadBlobs(ctx, blobSlot, 1, followerMultiaddr)
	downloadedBlobs = shared.EncodeBlobs(downloadedData)
	util.AssertBlobsEquals(blobs, downloadedBlobs)
}

func UploadBlobs(ctx context.Context, client *ethclient.Client, blobs types.Blobs) {
	chainId := big.NewInt(1)
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

	commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()

	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	txData := types.SignedBlobTx{
		Message: types.BlobTxMessage{
			ChainID:             view.Uint256View(*uint256.NewInt(chainId.Uint64())),
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
