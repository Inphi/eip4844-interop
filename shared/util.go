package shared

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func WaitForReceipt(ctx context.Context, client *ethclient.Client, txhash common.Hash) (*types.Receipt, error) {
	for {
		receipt, err := client.TransactionReceipt(ctx, txhash)
		if err == ethereum.NotFound {
			time.Sleep(time.Second * 1)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: TransactionReceipt", err)
		}
		return receipt, nil
	}
}
