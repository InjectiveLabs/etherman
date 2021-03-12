package deployer

import (
	"context"
	"encoding/json"
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
	// From is required there if calls need to be made
	From common.Address

	SolSource     string
	ContractName  string
	Contract      common.Address
	CoverageAgent CoverageDataCollector
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

	callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	var coverageTopic common.Hash
	var coverageEventABI abi.Event

	if d.options.EnableCoverage {
		_, coverageEventABI, err = d.GetCoverageEventInfo(callCtx, logsOpts.From, contract.Name, contract.Address)
		if err != ErrNoCoverage {
			if err != nil {
				return nil, err
			}

			coverageTopic = coverageEventABI.ID

			if logsOpts.CoverageAgent != nil {
				if err := logsOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}
				for _, statement := range contract.Statements {
					if statement[0] < 0 || statement[1] < 0 || statement[2] < 0 {
						continue
					}

					logsOpts.CoverageAgent.AddStatement(contract.Name,
						uint64(statement[0]),
						uint64(statement[1]),
						uint64(statement[2]),
					)
				}
			}
		}
	}

	callCtx, cancelFn = context.WithTimeout(context.Background(), d.options.CallTimeout)
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

	evABI, evABIFound := boundContract.ABI().Events[eventName]

	events = make([]interface{}, 0, len(receipt.Logs))
	for i, ethLog := range receipt.Logs {
		if ethLog == nil || len(ethLog.Topics) == 0 {
			continue
		}

		if d.options.EnableCoverage && ethLog.Topics[0] == coverageTopic {
			if logsOpts.CoverageAgent != nil {
				if err := logsOpts.CoverageAgent.CollectCoverageEvent(contract.Name, coverageEventABI, ethLog); err != nil {
					log.WithError(err).WithField("contract", contract.Name).Warningln("failed to collect coverage event from contract")
				}
			}

			continue
		} else if evABIFound && evABI.ID != ethLog.Topics[0] {
			continue
		}

		if eventUnpacker != nil {
			if !evABIFound {
				log.WithField("contract", logsOpts.ContractName).Errorf("event not found: %s", eventName)
				return nil, ErrEventNotFound
			}

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

		if evABIFound {
			eventOut := make(map[string]interface{})
			err := boundContract.UnpackLogIntoMap(eventOut, eventName, *ethLog)
			if err != nil {
				log.WithFields(log.Fields{
					"event": eventName,
					"index": i,
				}).WithError(err).Errorln("unable to unmarshal log")
				return nil, ErrEventParse
			}

			events = append(events, eventOut)
			continue
		}

		eventOut := make(map[string]interface{})
		ethLogData, _ := json.Marshal(ethLog)
		if err := json.Unmarshal(ethLogData, &eventOut); err != nil {
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
