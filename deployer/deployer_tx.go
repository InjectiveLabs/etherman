package deployer

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

type ContractTxOpts struct {
	From          common.Address
	FromPk        *ecdsa.PrivateKey
	SignerFn      bind.SignerFn
	SolSource     string
	ContractName  string
	Contract      common.Address
	Value         *big.Int
	BytecodeOnly  bool
	Await         bool
	CoverageAgent CoverageDataCollector
}

func (d *deployer) Tx(
	ctx context.Context,
	txOpts ContractTxOpts,
	methodName string,
	methodInputMapper AbiMethodInputMapperFunc,
) (txHash common.Hash, abiPackedCalldata []byte, err error) {
	solSourceFullPath, _ := filepath.Abs(txOpts.SolSource)
	contract := d.getCompiledContract(txOpts.ContractName, solSourceFullPath)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return noHash, nil, ErrCompilationFailed
	}
	contract.Address = txOpts.Contract

	if txOpts.BytecodeOnly {
		boundContract, err := BindContract(nil, contract)
		if err != nil {
			log.WithField("contract", txOpts.ContractName).WithError(err).Errorln("failed to bind contract")
			return noHash, nil, err
		}

		method, ok := boundContract.ABI().Methods[methodName]
		if !ok {
			log.WithField("contract", txOpts.ContractName).Errorf("method not found: %s", methodName)
			return noHash, nil, err
		}

		var mappedArgs []interface{}
		if methodInputMapper != nil {
			mappedArgs = methodInputMapper(method.Inputs)
		}

		abiPackedCalldata := append([]byte{}, method.ID...)
		packedArgs, err := method.Inputs.PackValues(mappedArgs)
		if err != nil {
			err = errors.Wrap(err, "failed to ABI-encode method args")
			return noHash, nil, err
		}
		abiPackedCalldata = append(abiPackedCalldata, packedArgs...)

		return noHash, abiPackedCalldata, nil
	}

	client, err := d.Backend()
	if err != nil {
		return noHash, nil, err
	}

	chainCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	chainId, err := client.ChainID(chainCtx)
	if err != nil {
		log.WithError(err).Errorln("failed get valid chain ID")
		return noHash, nil, ErrNoChainID
	}

	nonceCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	nonce, err := client.PendingNonceAt(nonceCtx, txOpts.From)
	if err != nil {
		log.WithField("from", txOpts.From.Hex()).WithError(err).Errorln("failed to get most recent nonce")
		return noHash, nil, ErrNoNonce
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", txOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return noHash, nil, err
	}

	callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	var coverageEventABI abi.Event
	var coverageTopic common.Hash

	if d.options.EnableCoverage {
		_, coverageEventABI, err = d.GetCoverageEventInfo(callCtx, txOpts.From, contract.Name, contract.Address)
		if err != ErrNoCoverage {
			if err != nil {
				return noHash, nil, err
			}

			coverageTopic = coverageEventABI.ID

			if txOpts.CoverageAgent != nil {
				if err := txOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}
				for _, statement := range contract.Statements {
					if statement[0] < 0 || statement[1] < 0 || statement[2] < 0 {
						continue
					}

					txOpts.CoverageAgent.AddStatement(contract.Name,
						uint64(statement[0]),
						uint64(statement[1]),
						uint64(statement[2]),
					)
				}
			}
		}
	}

	method, ok := boundContract.ABI().Methods[methodName]
	if !ok {
		log.WithField("contract", txOpts.ContractName).Errorf("method not found: %s", methodName)
		return noHash, nil, err
	}

	var mappedArgs []interface{}
	if methodInputMapper != nil {
		mappedArgs = methodInputMapper(method.Inputs)
	}

	boundContract.SetTransact(getTransactFn(client, contract.Address, &txHash))

	txCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	var signerFn bind.SignerFn
	if txOpts.SignerFn != nil {
		signerFn = txOpts.SignerFn
	} else {
		signerFn, err = getSignerFn(d.options.SignerType, chainId, txOpts.From, txOpts.FromPk)
		if err != nil {
			log.WithError(err).Errorln("failed to get signer function")
			return noHash, nil, err
		}
	}

	ethTxOpts := &bind.TransactOpts{
		From:     txOpts.From,
		Nonce:    big.NewInt(int64(nonce)),
		Signer:   signerFn,
		Value:    txOpts.Value,
		GasPrice: d.options.GasPrice,
		GasLimit: d.options.GasLimit,

		Context: txCtx,
	}

	txData, err := boundContract.Transact(ethTxOpts, methodName, mappedArgs...)
	if err != nil {
		if hasCoverageReport(err) {
			if txOpts.CoverageAgent != nil {
				coverageReportErr := txOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
				if coverageReportErr != nil {
					log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
				}
			}

			err = trimCoverageReport(err)
		}

		log.WithError(err).Errorln("failed to send transaction")
		return txHash, nil, err
	}

	if txOpts.Await || (d.options.EnableCoverage && txOpts.CoverageAgent != nil) {
		awaitCtx, cancelFn := context.WithTimeout(context.Background(), d.options.TxTimeout)
		defer cancelFn()

		log.WithField("contract", contract.Address.Hex()).Debugln("awaiting tx", txHash.Hex())

		blockNum, err := awaitTx(awaitCtx, client, txHash)

		if err == ErrTransactionReverted {
			// attempt to get reason
			reason, err := getRevertReason(ctx, txOpts.From, contract.Address, client, txData.Data(), blockNum)
			if err == nil && len(reason) > 0 {
				err = errors.New(reason)

				if hasCoverageReport(err) {
					if txOpts.CoverageAgent != nil {
						coverageReportErr := txOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
						if coverageReportErr != nil {
							log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
						}
					}

					err = trimCoverageReport(err)
				}

				return txHash, nil, err
			} else if err != nil {
				log.WithError(err).Warningln("failed to get revert reason")
				return txHash, nil, err
			}
		} else if err != nil {
			if hasCoverageReport(err) {
				if txOpts.CoverageAgent != nil {
					coverageReportErr := txOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
					if coverageReportErr != nil {
						log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
					}
				}

				err = trimCoverageReport(err)
			}

			return txHash, nil, err
		}
	}

	if d.options.EnableCoverage && txOpts.CoverageAgent != nil && txHash != noHash {
		callCtx, cancelFn = context.WithTimeout(context.Background(), d.options.CallTimeout)
		defer cancelFn()

		callLog := log.WithField("txHash", txHash.Hex())
		receipt, err := client.TransactionReceipt(callCtx, txHash)
		if err != nil {
			if err == ethereum.NotFound {
				callLog.Errorln("unable to collect coverage: transaction not found")
				return txHash, nil, ErrTxNotFound
			}

			callLog.WithError(err).Errorln("failed to get transaction receipt")
			return txHash, nil, err
		}

		for _, ethLog := range receipt.Logs {
			if ethLog == nil || len(ethLog.Topics) == 0 {
				continue
			} else if ethLog.Topics[0] == coverageTopic {
				if txOpts.CoverageAgent != nil {
					if err := txOpts.CoverageAgent.CollectCoverageEvent(contract.Name, coverageEventABI, ethLog); err != nil {
						log.WithError(err).WithField("contract", contract.Name).Warningln("failed to collect coverage event from contract")
					}
				}

				continue
			}
		}
	}

	return txHash, nil, err
}
