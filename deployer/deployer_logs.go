package deployer

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ctypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrEventParse          = errors.New("unable to unmarshal log")
	ErrTxNotFound          = errors.New("transaction not found")
	ErrTransactionReverted = errors.New("transaction reverted without logs")
)

type ContractLogsOpts struct {
	SolSource    string
	ContractName string
	Contract     common.Address
}

type LogUnpacker interface {
	UnpackLog(out interface{}, event string, log ctypes.Log) error
}

type ContractLogUnpackFunc func(unpacker LogUnpacker, event abi.Event, log ctypes.Log) (interface{}, error)

func (d *deployer) Logs(
	ctx context.Context,
	logsOpts ContractLogsOpts,
	txHash common.Hash,
	eventName string,
	eventUnpacker ContractLogUnpackFunc,
) (events []interface{}, err error) {
	solSourceFullPath, _ := filepath.Abs(logsOpts.SolSource)
	contract := d.getCompiledContract(logsOpts.ContractName, solSourceFullPath)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return nil, ErrCompilationFailed
	}

	contract.Address = logsOpts.Contract

	client, err := d.Backend()
	if err != nil {
		return nil, err
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", logsOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return nil, err
	}

	evABI, ok := boundContract.ABI().Events[eventName]
	if !ok {
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

	events = make([]interface{}, 0, len(receipt.Logs))
	for i, ethLog := range receipt.Logs {
		if ethLog == nil || len(ethLog.Topics) == 0 {
			continue
		} else if evABI.ID != ethLog.Topics[0] {
			continue
		}

		if eventUnpacker != nil {
			ev, err := eventUnpacker(boundContract, evABI, *ethLog)
			if err != nil {
				log.WithFields(log.Fields{
					"event": eventName,
					"index": i,
				}).WithError(err).Errorln("unable to unmarshal log")
				return nil, ErrEventParse
			}

			events = append(events, ev)
			continue
		}

		var eventOut map[string]interface{}
		err := boundContract.UnpackLogIntoMap(eventOut, eventName, *ethLog)
		if err != nil {
			log.WithFields(log.Fields{
				"event": eventName,
				"index": i,
			}).WithError(err).Errorln("unable to unmarshal log")
			return nil, ErrEventParse
		}

		events = append(events, eventOut)
	}

	return events, nil
}
