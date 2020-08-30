package main

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

type Client struct {
	*ethclient.Client

	rc *rpc.Client
}

func NewClient(rc *rpc.Client) *Client {
	return &Client{
		Client: ethclient.NewClient(rc),
		rc:     rc,
	}
}

func (ec *Client) SendTransactionWithRet(ctx context.Context, tx *types.Transaction) (txHash common.Hash, err error) {
	data, err := rlp.EncodeToBytes(tx)
	if err != nil {
		return common.Hash{}, err
	}

	if err := ec.rc.CallContext(ctx, &txHash, "eth_sendRawTransaction", hexutil.Encode(data)); err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}
