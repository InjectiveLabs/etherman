package deployer

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrTxNotFound          = errors.New("transaction not found")
	ErrTransactionReverted = errors.New("transaction reverted without logs")
)

type ContractLogsOpts struct {
	EVMEndpoint  string
	SolSource    string
	ContractName string
	Contract     common.Address
}

type ContractEvent map[string]interface{}

func (d *deployer) Logs(
	ctx context.Context,
	logsOpts ContractLogsOpts,
	txHash common.Hash,
	eventName string,
) (events []ContractEvent, err error) {
	solSourceFullPath, _ := filepath.Abs(logsOpts.SolSource)
	contract := d.getCompiledContract(logsOpts.ContractName, solSourceFullPath, true)
	contract.Address = logsOpts.Contract

	dialCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	var client *Client
	rc, err := rpc.DialContext(dialCtx, logsOpts.EVMEndpoint)
	if err != nil {
		log.WithError(err).Errorln("failed to dial EVM RPC endpoint")
		return nil, ErrEndpointUnreachable
	} else {
		client = NewClient(rc)
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", logsOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return nil, err
	}

	if _, ok := boundContract.ABI().Events[eventName]; !ok {
		log.WithField("contract", logsOpts.ContractName).Errorf("event not found: %s", eventName)
		return nil, ErrEventNotFound
	}

	callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	callLog := log.WithField("txHash", txHash.Hex())
	receipt, err := client.TransactionReceipt(callCtx, txHash)
	if err != nil {
		if err == ethereum.NotFound {
			callLog.Errorln("transaction not found")
			return nil, ErrTxNotFound
		}

		callLog.WithError(err).Errorln("failed to get transaction receipt")
		return nil, err
	} else if receipt.Status != 1 {
		callLog.Errorln("transaction reverted without logs")
		return nil, ErrTransactionReverted
	}

	events = make([]ContractEvent, 0, len(receipt.Logs))
	for _, ethLog := range receipt.Logs {
		out := make(map[string]interface{})
		if err := boundContract.UnpackLogIntoMap(out, eventName, *ethLog); err == nil {
			events = append(events, out)
		} else {
			log.WithField("event", eventName).WithError(err).Warningln("unable to unmarshal log")
		}
	}

	return events, nil
}
