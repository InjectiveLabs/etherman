package deployer

import (
	"context"
	"crypto/ecdsa"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

type AbiMethodInputMapperFunc func(args abi.Arguments) []interface{}

type ContractCallOpts struct {
	From          common.Address
	SolSource     string
	ContractName  string
	Contract      common.Address
	CoverageCall  ContractCoverageCallOpts
	CoverageAgent CoverageDataCollector
}

type ContractCoverageCallOpts struct {
	FromPk *ecdsa.PrivateKey
}

func (d *deployer) Call(
	ctx context.Context,
	callOpts ContractCallOpts,
	methodName string,
	methodInputMapper AbiMethodInputMapperFunc,
) (output []interface{}, outputAbi abi.Arguments, err error) {
	solSourceFullPath, _ := filepath.Abs(callOpts.SolSource)
	contract := d.getCompiledContract(callOpts.ContractName, solSourceFullPath)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return nil, nil, ErrCompilationFailed
	}
	contract.Address = callOpts.Contract

	client, err := d.Backend()
	if err != nil {
		return nil, nil, err
	}

	chainCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	if _, err := client.ChainID(chainCtx); err != nil {
		log.WithError(err).Errorln("failed get valid chain ID")
		return nil, nil, err
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", callOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return nil, nil, err
	}

	callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	method, ok := boundContract.ABI().Methods[methodName]
	if !ok {
		log.WithField("contract", callOpts.ContractName).Errorf("method not found: %s", methodName)
		return nil, nil, err
	}

	var mappedArgs []interface{}
	if methodInputMapper != nil {
		mappedArgs = methodInputMapper(method.Inputs)
	}

	callCtx, cancelFn = context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	ethCallOpts := &bind.CallOpts{
		From:    callOpts.From,
		Context: callCtx,
	}

	if d.options.EnableCoverage {
		var coverageTopic common.Hash
		var coverageEventABI abi.Event

		_, coverageEventABI, err = d.GetCoverageEventInfo(callCtx, callOpts.From, contract.Name, contract.Address)
		if err != ErrNoCoverage {
			if err != nil {
				return nil, nil, err
			}

			coverageTopic = coverageEventABI.ID

			if callOpts.CoverageAgent != nil {
				if err := callOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}
				for _, statement := range contract.Statements {
					if statement[0] < 0 || statement[1] < 0 || statement[2] < 0 {
						continue
					}

					callOpts.CoverageAgent.AddStatement(contract.Name,
						uint64(statement[0]),
						uint64(statement[1]),
						uint64(statement[2]),
					)
				}
			}
		}

		if callOpts.CoverageAgent != nil && coverageTopic != noHash {
			if callOpts.CoverageCall.FromPk == nil {
				err := errors.New("call with enabled coverage, but no PrivKey provided (for tx)")
				return nil, method.Outputs, err
			}

			txOpts := ContractTxOpts{
				From:          callOpts.From,
				FromPk:        callOpts.CoverageCall.FromPk,
				SolSource:     callOpts.SolSource,
				ContractName:  callOpts.ContractName,
				Contract:      callOpts.Contract,
				CoverageAgent: callOpts.CoverageAgent,
				Await:         true,
			}

			txHash, _, err := d.Tx(context.Background(), txOpts, methodName, methodInputMapper)
			if err != nil {
				if hasCoverageReport(err) {
					if callOpts.CoverageAgent != nil {
						coverageReportErr := callOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
						if coverageReportErr != nil {
							log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
						}
					}

					err = trimCoverageReport(err)
				}

				return nil, method.Outputs, err
			}

			awaitCtx, cancelFn := context.WithTimeout(context.Background(), d.options.TxTimeout)
			defer cancelFn()

			blockNum, err := awaitTx(awaitCtx, client, txHash)
			ethCallOpts.BlockNumber = blockNum

			if err := boundContract.Call(ethCallOpts, &output, methodName, mappedArgs...); err != nil {
				log.WithError(err).Errorln("failed to call contract method")
				err = errors.Wrap(err, "failed to call contract method")
				return nil, method.Outputs, err
			}

			return output, method.Outputs, nil
		}
	}

	// a simple call
	if err := boundContract.Call(ethCallOpts, &output, methodName, mappedArgs...); err != nil {
		if hasCoverageReport(err) {
			if callOpts.CoverageAgent != nil {
				coverageReportErr := callOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
				if coverageReportErr != nil {
					log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
				}
			}

			err = trimCoverageReport(err)
		}

		err = errors.Wrap(err, "failed to call contract method")
		return nil, method.Outputs, err
	}

	return output, method.Outputs, nil
}
