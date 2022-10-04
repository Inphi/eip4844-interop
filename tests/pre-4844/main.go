package main

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/Inphi/eip4844-interop/shared"
	"github.com/Inphi/eip4844-interop/tests/ctrl"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// Asserts that transaction still work before the 4844 fork in execution
func main() {
	ctrl.InitE2ETest()

	chainId := big.NewInt(1)
	signer := types.NewDankSigner(chainId)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	client := ctrl.Env.EthClient
	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	msg := ethereum.CallMsg{
		From:  crypto.PubkeyToAddress(key.PublicKey),
		To:    &common.Address{},
		Gas:   21000,
		Value: big.NewInt(10000),
	}
	gas, err := client.EstimateGas(ctx, msg)
	if err != nil {
		log.Fatalf("EstimateGas error: %v", err)
	}

	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	tx, err := types.SignTx(types.NewTransaction(nonce, to, big.NewInt(10000), params.TxGas, big.NewInt(int64(gas)), nil), signer, key)
	if err != nil {
		log.Fatalf("Error signing tx: %v", err)
	}

	err = client.SendTransaction(ctx, tx)
	if err != nil {
		log.Fatalf("Error sending tx: %v", err)
	}

	log.Printf("Waiting for transaction (%v) to be included...", tx.Hash())
	var receipt *types.Receipt
	for {
		receipt, err = client.TransactionReceipt(ctx, tx.Hash())
		if err == ethereum.NotFound {
			time.Sleep(time.Second * 1)
			continue
		}
		if err != nil {
			log.Fatalf("Error getting tx receipt for %v: %v", tx.Hash(), err)
		}
		break
	}

	blockHash := receipt.BlockHash.Hex()
	_, err = client.BlockByHash(ctx, common.HexToHash(blockHash))
	if err != nil {
		log.Fatalf("Error getting block: %v", err)
	}

	eip4844Block := ctrl.Env.GethChainConfig.ShardingForkBlock.Uint64()
	if receipt.BlockNumber.Uint64() > eip4844Block {
		// TODO: Avoid this issue by configuring the chain config at runtime
		log.Fatalf("Test condition violation. Transaction must be included before eip4844 fork. Check the geth chain config")
	}

}
