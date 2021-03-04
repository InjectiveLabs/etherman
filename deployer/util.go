package deployer

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/InjectiveLabs/evm-deploy-contract/sol"
)

var ErrAwaitTimeout = errors.New("await timeout")

func awaitTx(ctx context.Context, client *Client, txHash common.Hash) error {
	awaitLog := log.WithField("hash", txHash.Hex())
	awaitLog.Infoln("awaiting transaction")

	for {
		select {
		case <-ctx.Done():
			return ErrAwaitTimeout
		default:
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err != nil {
				if err == ethereum.NotFound {
					time.Sleep(time.Second)
					continue
				}

				awaitLog.WithError(err).Errorln("failed to await transaction")
				return err
			}

			if receipt.Status == 0 {
				awaitLog.Errorln("transaction reverted")
				return ErrTransactionReverted
			}

			// all good
			return nil
		}
	}
}

func (d *deployer) getCompiledContract(contractName, solFullPath string, loadFromCache bool) *sol.Contract {
	if !d.options.NoCache && loadFromCache {
		cacheLog := log.WithField("path", d.options.BuildCacheDir)

		cache, err := NewBuildCache(d.options.BuildCacheDir)
		if err != nil {
			cacheLog.WithError(err).Warningln("failed to use build cache dir")
		} else {
			contract, err := cache.LoadContract(solFullPath, contractName)
			if err != nil && err != ErrNoCache {
				cacheLog.WithError(err).Warningln("failed to use build cache")
			} else {
				return contract
			}
		}
	}

	ts := time.Now()

	contracts, err := d.compiler.Compile(filepath.Dir(solFullPath), filepath.Base(solFullPath), 200)
	if err != nil {
		log.WithFields(log.Fields{
			"dir":  filepath.Dir(solFullPath),
			"file": filepath.Base(solFullPath),
		}).WithError(err).Errorln("failed to compile .sol files")

		return nil
	}

	log.Debugln("compiled sources in", time.Since(ts))

	for name := range contracts {
		log.Debugln("found", name, "contract")
	}

	var contract *sol.Contract
	for name, c := range contracts {
		if name == contractName {
			contract = c
		}
	}

	if contract == nil {
		log.WithField("contract", contractName).Errorln("specified contract not found in compiled sources")
		return nil
	}

	return contract
}

type SignerType string

const (
	SignerEIP155    SignerType = "eip155"
	SignerHomestead SignerType = "homestead"
)

func getSignerFn(
	signerType SignerType,
	chainId *big.Int,
	from common.Address,
	pk *ecdsa.PrivateKey,
) (bind.SignerFn, error) {
	switch signerType {
	case SignerEIP155:
		opts, err := bind.NewKeyedTransactorWithChainID(pk, chainId)
		if err != nil {
			err = errors.Wrap(err, "failed to init NewKeyedTransactorWithChainID")
			return nil, err
		}

		return opts.Signer, nil

	case SignerHomestead:
		signerFn := func(address common.Address, tx *types.Transaction) (*types.Transaction, error) {
			if address != from {
				err := errors.Errorf("not authorized to sign with %s", address.Hex())
				return nil, err
			}

			signer := &types.HomesteadSigner{}
			txHash := signer.Hash(tx)
			log.Printf("signer: %T", signer)
			log.Println("signer obtained tx hash:", txHash.Hex())

			signature, err := crypto.Sign(txHash.Bytes(), pk)
			if err != nil {
				return nil, err
			}

			return tx.WithSignature(signer, signature)
		}

		return signerFn, nil

	default:
		err := errors.Errorf("unsupported signer type: %s", signerType)
		return nil, err
	}
}

func getTransactFn(ec *Client, contractAddress common.Address, txHashOut *common.Hash) TransactFunc {
	return func(opts *bind.TransactOpts, contract *common.Address, input []byte) (*types.Transaction, error) {
		var err error

		// Ensure a valid value field and resolve the account nonce
		value := opts.Value
		if value == nil {
			value = new(big.Int)
		}
		var nonce uint64
		if opts.Nonce == nil {
			nonce, err = ec.PendingNonceAt(opts.Context, opts.From)
			if err != nil {
				return nil, fmt.Errorf("failed to retrieve account nonce: %v", err)
			}
		} else {
			nonce = opts.Nonce.Uint64()
		}
		// Figure out the gas allowance and gas price values
		gasPrice := opts.GasPrice
		if gasPrice == nil {
			gasPrice, err = ec.SuggestGasPrice(opts.Context)
			if err != nil {
				return nil, fmt.Errorf("failed to suggest gas price: %v", err)
			}
		}
		gasLimit := opts.GasLimit
		if gasLimit == 0 {
			// Gas estimation cannot succeed without code for method invocations
			if contract != nil {
				if code, err := ec.PendingCodeAt(opts.Context, contractAddress); err != nil {
					return nil, err
				} else if len(code) == 0 {
					return nil, bind.ErrNoCode
				}
			}
			// If the contract surely has code (or code is not needed), estimate the transaction
			msg := ethereum.CallMsg{From: opts.From, To: contract, GasPrice: gasPrice, Value: value, Data: input}
			gasLimit, err = ec.EstimateGas(opts.Context, msg)
			if err != nil {
				return nil, fmt.Errorf("failed to estimate gas needed: %v", err)
			}
		}
		// Create the transaction, sign it and schedule it for execution
		var rawTx *types.Transaction
		if contract == nil {
			rawTx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, input)
		} else {
			rawTx = types.NewTransaction(nonce, contractAddress, value, gasLimit, gasPrice, input)
		}
		if opts.Signer == nil {
			return nil, errors.New("no signer to authorize the transaction with")
		}
		signedTx, err := opts.Signer(opts.From, rawTx)
		if err != nil {
			return nil, err
		}

		txHash, err := ec.SendTransactionWithRet(opts.Context, signedTx)
		if err != nil {
			*txHashOut = txHash
			return nil, err
		}

		*txHashOut = txHash
		return signedTx, nil
	}
}
