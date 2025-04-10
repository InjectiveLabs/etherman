package deployer

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/etherman/sol"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrEndpointUnreachable = errors.New("unable to dial EVM RPC endpoint")
	ErrNoChainID           = errors.New("failed to get valid Chain ID")
	ErrNoNonce             = errors.New("failed to get latest from nonce")
)

type ContractDeployOpts struct {
	From          common.Address
	FromPk        *ecdsa.PrivateKey
	SignerFn      bind.SignerFn
	SolSource     string
	ContractName  string
	BytecodeOnly  bool
	Await         bool
	CoverageAgent CoverageDataCollector
}

func (d *deployer) Deploy(
	ctx context.Context,
	deployOpts ContractDeployOpts,
	constructorInputMapper AbiMethodInputMapperFunc,
) (txHash common.Hash, contract *sol.Contract, err error) {
	solSourceFullPath, _ := filepath.Abs(deployOpts.SolSource)
	contract = d.getCompiledContract(deployOpts.ContractName, solSourceFullPath)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return noHash, nil, ErrCompilationFailed
	}

	if deployOpts.BytecodeOnly {
		boundContract, err := BindContract(nil, contract)
		if err != nil {
			log.WithField("contract", deployOpts.ContractName).WithError(err).Errorln("failed to bind contract")
			return noHash, nil, err
		}

		var mappedArgs []interface{}
		if constructorInputMapper != nil {
			mappedArgs = constructorInputMapper(boundContract.ABI().Constructor.Inputs)
		}

		abiPackedArgs, err := boundContract.ABI().Constructor.Inputs.PackValues(mappedArgs)
		if err != nil {
			err = errors.Wrap(err, "failed to ABI-encode constructor values")
			return noHash, nil, err
		}

		contract.Bin = contract.Bin + hex.EncodeToString(abiPackedArgs)
		return noHash, contract, nil
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

	nonce, err := client.NonceAt(nonceCtx, deployOpts.From, nil)
	if err != nil {
		log.WithField("from", deployOpts.From.Hex()).WithError(err).Errorln("failed to get most recent nonce")
		return noHash, nil, ErrNoNonce
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", deployOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return noHash, nil, err
	}

	var mappedArgs []interface{}
	if constructorInputMapper != nil {
		mappedArgs = constructorInputMapper(boundContract.ABI().Constructor.Inputs)
	}

	boundContract.SetTransact(getTransactFn(client, common.Address{}, &txHash))

	txCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	var signerFn bind.SignerFn
	if deployOpts.SignerFn != nil {
		signerFn = deployOpts.SignerFn
	} else {
		signerFn, err = getSignerFn(d.options.SignerType, chainId, deployOpts.From, deployOpts.FromPk)
		if err != nil {
			log.WithError(err).Errorln("failed to get signer function")
			return noHash, nil, err
		}
	}

	ethTxOpts := &bind.TransactOpts{
		From:     deployOpts.From,
		Nonce:    big.NewInt(int64(nonce)),
		Signer:   signerFn,
		Value:    big.NewInt(0),
		GasPrice: d.options.GasPrice,
		GasLimit: d.options.GasLimit,

		Context: txCtx,
	}

	log.WithFields(log.Fields{
		"nonce":    big.NewInt(int64(nonce)),
		"gasPrice": d.options.GasPrice.String(),
		"gasLimit": d.options.GasLimit,
	}).Debugln("deploying contract", contract.Name)

	address, _, err := boundContract.DeployContract(ethTxOpts, mappedArgs...)
	if err != nil {
		if hasCoverageReport(err) {
			if deployOpts.CoverageAgent != nil {
				if err := deployOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}

				coverageReportErr := deployOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
				if coverageReportErr != nil {
					log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
				}
			}

			err = trimCoverageReport(err)
		}

		log.WithError(err).WithField("txHash", txHash.Hex()).Errorln("failed to deploy contract")
		return txHash, nil, err
	}
	contract.Address = address

	if deployOpts.Await || (d.options.EnableCoverage && deployOpts.CoverageAgent != nil) {
		awaitCtx, cancelFn := context.WithTimeout(context.Background(), d.options.TxTimeout)
		defer cancelFn()

		log.WithField("txHash", txHash.Hex()).Debugln("awaiting contract deployment", address.Hex())

		_, err = awaitTx(awaitCtx, client, txHash)
	}
	if err != nil {
		if hasCoverageReport(err) {
			if deployOpts.CoverageAgent != nil {
				if err := deployOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}

				coverageReportErr := deployOpts.CoverageAgent.CollectCoverageRevert(contract.Name, err)
				if coverageReportErr != nil {
					log.WithError(coverageReportErr).Warningln("failed to collect coverage revert event")
				}
			}

			err = trimCoverageReport(err)
		}

		return txHash, contract, err
	}

	if d.options.EnableCoverage && deployOpts.CoverageAgent != nil && txHash != noHash {
		callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
		defer cancelFn()

		var coverageTopic common.Hash
		_, coverageEventABI, err := d.GetCoverageEventInfo(callCtx, deployOpts.From, contract.Name, contract.Address)
		if err != ErrNoCoverage {
			if err != nil {
				return txHash, contract, err
			}

			coverageTopic = coverageEventABI.ID

			if deployOpts.CoverageAgent != nil {
				if err := deployOpts.CoverageAgent.LoadContract(contract); err != nil {
					log.WithError(err).Errorln("failed to open referenced dependecies for coverage reporting")
				}
				for _, statement := range contract.Statements {
					if statement[0] < 0 || statement[1] < 0 || statement[2] < 0 {
						continue
					}

					deployOpts.CoverageAgent.AddStatement(contract.Name,
						uint64(statement[0]),
						uint64(statement[1]),
						uint64(statement[2]),
					)
				}
			}
		}

		if coverageTopic == noHash {
			return txHash, contract, err
		}

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
				if deployOpts.CoverageAgent != nil {
					if err := deployOpts.CoverageAgent.CollectCoverageEvent(contract.Name, coverageEventABI, ethLog); err != nil {
						log.WithError(err).WithField("contract", contract.Name).Warningln("failed to collect coverage event from contract")
					}
				}

				continue
			}
		}
	}

	return txHash, contract, err
}

var noHash = common.Hash{}
