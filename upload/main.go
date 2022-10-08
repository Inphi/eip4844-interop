package main

import (
	"context"
	"flag"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
)

func main() {
	prv := "45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"
	addr := "http://localhost:8545"

	before := flag.Uint64("before", 0, "Block to wait for before submitting transaction")
	after := flag.Uint64("after", 0, "Block to wait for after submitting transaction")
	flag.Parse()

	file := flag.Arg(0)
	if file == "" {
		log.Fatalf("File parameter missing")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	chainId := big.NewInt(1)
	signer := types.NewDankSigner(chainId)

	ctx := context.Background()
	client, err := ethclient.DialContext(ctx, addr)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}

	key, err := crypto.HexToECDSA(prv)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	if *before > 0 {
		waitForBlock(ctx, client, *before)
	}

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	blobs := shared.EncodeBlobs(data)
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

	if *after > 0 {
		waitForBlock(ctx, client, *after)
	}
}

func waitForBlock(ctx context.Context, client *ethclient.Client, block uint64) {
	for {
		bn, err := client.BlockNumber(ctx)
		if err != nil {
			log.Fatalf("Error requesting block number: %v", err)
		}
		if bn >= block {
			return
		}
		log.Printf("Waiting for block %d, current %d", block, bn)
		time.Sleep(1 * time.Second)
	}
}
