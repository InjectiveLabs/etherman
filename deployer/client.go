package deployer

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
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
		return tx.Hash(), err
	}

	return txHash, nil
}

var ErrClientNotAvailable = errors.New("EVM RPC client is not available due to connection issue")

func (d *deployer) Backend() (*Client, error) {
	d.initClientOnce.Do(func() {
		dialCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
		defer cancelFn()

		rc, err := rpc.DialContext(dialCtx, d.options.EVMRPCEndpoint)
		if err != nil {
			log.WithError(err).Errorln("failed to dial EVM RPC endpoint")
			return
		}

		d.client = NewClient(rc)
	})

	if d.client == nil {
		return nil, ErrClientNotAvailable
	}

	return d.client, nil
}
