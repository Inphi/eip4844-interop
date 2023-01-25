package main

import (
	"context"
	"log"
	"math/big"
	"os"
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
	clientName := "prysm"
	if len(os.Args) > 1 {
		clientName = os.Args[1]
	}

	ctrl.InitE2ETest(clientName)

	env := ctrl.GetEnv()
	chainId := env.GethChainConfig.ChainID
	signer := types.NewDankSigner(chainId)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*20)
	defer cancel()

	client, err := ctrl.GetExecutionClient(ctx)
	if err != nil {
		log.Fatalf("unable to get execution client: %v", err)
	}

	key, err := crypto.HexToECDSA(shared.PrivateKey)
	if err != nil {
		log.Fatalf("Failed to load private key: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, crypto.PubkeyToAddress(key.PublicKey))
	if err != nil {
		log.Fatalf("Error getting nonce: %v", err)
	}
	log.Printf("Nonce: %d", nonce)

	gasTipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		log.Fatalf("Suggest gas tip cap: %v", err)
	}
	gasFeeCap, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("Suggest gas fee price: %v", err)
	}

	to := common.HexToAddress("ffb38a7a99e3e2335be83fc74b7faa19d5531243")
	tx, err := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &to,
		Value:     big.NewInt(10000),
		Gas:       params.TxGas,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
	}), signer, key)
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
	blk, err := client.BlockByHash(ctx, common.HexToHash(blockHash))
	if err != nil {
		log.Fatalf("Error getting block: %v", err)
	}

	shardingForkTime := ctrl.GetEnv().GethChainConfig.ShardingForkTime
	if shardingForkTime == nil {
		log.Fatalf("shardingForkTime is not set in configuration")
	}
	eip4844ForkTime := *shardingForkTime
	if blk.Time() > eip4844ForkTime {
		// TODO: Avoid this issue by configuring the chain config at runtime
		log.Fatalf("Test condition violation. Transaction must be included before eip4844 fork. Check the geth chain config")
	}

}
