package main

import (
	"context"
	"log"
	"math/big"
	"sync"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/ethereum/go-ethereum"
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

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	// send dummy data
	blobs := shared.EncodeBlobs([]byte("EKANS"))
	commitments, versionedHashes, aggregatedProof, err := blobs.ComputeCommitmentsAndAggregatedProof()

	// Send multiple transactions asynchronously to induce non-zero excess_blobs
	var txs []*types.Transaction
	numTxs := 20
	for i := 0; i < numTxs; i++ {
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
			log.Printf("Transaction submitted. hash=%v", tx.Hash())
		}()
	}
	wg.Wait()

	log.Printf("Waiting for transactions to be included...")
	receiptBlockHashes := make(map[string]bool)
	for _, tx := range txs {
		for {
			receipt, err := client.TransactionReceipt(ctx, tx.Hash())
			if err == ethereum.NotFound {
				continue
			}
			if err != nil {
				log.Fatalf("Error getting tx receipt for %v: %v", tx.Hash(), err)
			}
			receiptBlockHashes[receipt.BlockHash.Hex()] = true
			break
		}
	}

	for blockHash, _ := range receiptBlockHashes {
		block, err := client.BlockByHash(ctx, common.HexToHash(blockHash))
		if err != nil {
			log.Fatalf("Error getting block: %v", err)
		}
		if block.ExcessBlobs() != 0 {
			log.Printf("non-zero ExcessBlobs found. block_hash=%v excess_blobs=%v", blockHash, block.ExcessBlobs())
			return
		}
	}
	log.Fatal("Failed to find a block containing non-zero excess_blobs")
}
