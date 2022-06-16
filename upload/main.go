package main

import (
	"context"
	"flag"
	"github.com/ethereum/go-ethereum/params"
	"log"
	"math/big"
	"os"

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

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	blobs := encodeBlobs(data)
	var commitments []types.KZGCommitment
	var hashes []common.Hash
	for _, b := range blobs {
		c, ok := b.ComputeCommitment()
		if !ok {
			panic("Could not compute commitment")
		}
		commitments = append(commitments, c)
		hashes = append(hashes, c.ComputeVersionedHash())
	}
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
			BlobVersionedHashes: hashes,
		},
	}

	wrapData := types.BlobTxWrapData{
		BlobKzgs: commitments,
		Blobs:    blobs,
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

	log.Printf("Done")
}

func encodeBlobs(data []byte) []types.Blob {
	blobs := []types.Blob{{}}
	blobIndex := 0
	fieldIndex := -1
	for i := 0; i < len(data); i += 31 {
		fieldIndex++
		if fieldIndex == params.FieldElementsPerBlob {
			blobs = append(blobs, types.Blob{})
			blobIndex++
			fieldIndex = 0
		}
		max := i + 31
		if max > len(data) {
			max = len(data)
		}
		copy(blobs[blobIndex][fieldIndex][:], data[i:max])
	}
	return blobs
}
