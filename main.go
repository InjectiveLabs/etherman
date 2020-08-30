package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	cli "github.com/jawher/mow.cli"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/InjectiveLabs/evm-deploy-contract/sol"
)

var app = cli.App("evm-deploy-contract", "Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x")

var (
	solcPathSet bool
	solcPath    = app.String(cli.StringOpt{
		Name:      "solc-path",
		Desc:      "Set path solc executable. Found using 'which' otherwise",
		EnvVar:    "SOLC_PATH",
		Value:     "",
		SetByUser: &solcPathSet,
	})

	contractName = app.String(cli.StringOpt{
		Name:   "N name",
		Desc:   "Specify contract name to use.",
		EnvVar: "SOL_CONTRACT_NAME",
		Value:  "Counter",
	})

	solSource = app.String(cli.StringOpt{
		Name:   "S source",
		Desc:   "Set path for .sol source file of the contract.",
		EnvVar: "SOL_SOURCE_FILE",
		Value:  "contracts/Counter.sol",
	})

	evmEndpoint = app.String(cli.StringOpt{
		Name:   "E endpoint",
		Desc:   "Specify HTTP URI for EVM JSON-RPC endpoint",
		EnvVar: "EVM_RPC_HTTP",
		Value:  "http://localhost:8545",
	})

	fromPrivkey = app.String(cli.StringOpt{
		Name:   "P privkey",
		Desc:   "Provide hex-encoded private key for tx signing.",
		EnvVar: "TX_FROM_PRIVKEY",
		Value:  "",
	})

	signerType = app.String(cli.StringOpt{
		Name:   "signer",
		Desc:   "Override the default signer with other supported: homestead, eip155",
		EnvVar: "TX_SIGNER",
		Value:  "eip155",
	})

	gasPriceSet bool
	gasPrice    = app.Int(cli.IntOpt{
		Name:      "G gas-price",
		Desc:      "Override estimated gas price with this option.",
		EnvVar:    "TX_GAS_PRICE",
		Value:     50, // wei
		SetByUser: &gasPriceSet,
	})

	gasLimit = app.Int(cli.IntOpt{
		Name:   "L gas-limit",
		Desc:   "Set the maximum gas for tx.",
		EnvVar: "TX_GAS_LIMIT",
		Value:  5000000,
	})
)

type SignerType string

const (
	SignerEIP155    SignerType = "eip155"
	SignerHomestead SignerType = "homestead"
)

const defaultRPCTimeout = 10 * time.Second

func main() {
	app.Action = runApp
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runApp() {
	var solc sol.Compiler
	var err error
	if solcPathSet {
		if solc, err = sol.NewSolCompiler(*solcPath); err != nil {
			log.WithField("path", *solcPath).WithError(err).Fatal("failed to find solc compiler at path")
		}
	} else {
		solcPathFound, err := sol.WhichSolc()
		if err != nil {
			log.WithError(err).Fatal("failed to find solc compiler")
		}

		if solc, err = sol.NewSolCompiler(solcPathFound); err != nil {
			log.WithField("path", solcPathFound).WithError(err).Fatal("failed to find solc compiler at path")
		}
	}

	solSourceFullPath, _ := filepath.Abs(*solSource)
	contracts, err := solc.Compile(filepath.Dir(solSourceFullPath), filepath.Base(solSourceFullPath), 200)
	if err != nil {
		log.WithFields(log.Fields{
			"dir":  filepath.Dir(solSourceFullPath),
			"file": filepath.Base(solSourceFullPath),
		}).WithError(err).Fatal("failed to compile .sol files")
	}

	for name := range contracts {
		log.Infoln("found", name, "contract")
	}

	var contract *sol.Contract
	for name, c := range contracts {
		if name == *contractName {
			contract = c
		}
	}

	if contract == nil {
		log.WithField("contract", *contractName).Fatal("specified contract not found in compiled sources")
	}

	pkHex := *fromPrivkey
	if len(pkHex) == 0 {
		log.Fatal("private key not specified, use -P or --privkey")
	} else {
		pkHex = strings.TrimPrefix(pkHex, "0x")
	}

	privateKey, err := crypto.HexToECDSA(pkHex)
	if err != nil {
		log.WithError(err).Fatal("failed to convert privkey from hex to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	log.Infoln("sending from", fromAddress.Hex())

	dialCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancelFn()

	var client *Client
	rc, err := rpc.DialContext(dialCtx, *evmEndpoint)
	if err != nil {
		log.WithError(err).Fatal("failed to dial EVM RPC endpoint")
	} else {
		client = NewClient(rc)
	}

	chainCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancelFn()

	chainId, err := client.ChainID(chainCtx)
	if err != nil {
		log.WithError(err).Fatal("failed get valid chain ID")
	} else {
		log.Println("using chain ID:", chainId.String())
	}

	nonceCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancelFn()

	nonce, err := client.PendingNonceAt(nonceCtx, fromAddress)
	if err != nil {
		log.WithField("from", fromAddress.Hex()).WithError(err).Fatal("failed to get most recent nonce")
	}

	var gasPriceInt *big.Int
	if gasPriceSet {
		gasPriceInt = big.NewInt(int64(*gasPrice))
	} else {
		gasCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancelFn()

		price, err := client.SuggestGasPrice(gasCtx)
		if err != nil {
			log.WithError(err).Fatal("failed to estimate gas on the evm node")
		}

		gasPriceInt = price
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", *contractName).WithError(err).Fatal("failed bind contract")
	}

	var txHash common.Hash
	boundContract.SetTransact(getTransactFn(client, common.Address{}, &txHash))

	txCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
	defer cancelFn()

	txOpts := &bind.TransactOpts{
		From:     fromAddress,
		Nonce:    big.NewInt(int64(nonce)),
		Signer:   getSignerFn(SignerType(*signerType), chainId, fromAddress, privateKey),
		Value:    big.NewInt(0),
		GasPrice: gasPriceInt,
		GasLimit: uint64(*gasLimit),

		Context: txCtx,
	}

	address, _, err := boundContract.DeployContract(txOpts)
	if err != nil {
		log.WithError(err).Fatalln("failed to deploy contract")
		return
	}

	log.Infoln("contract address:", address.Hex())
	log.Infoln("tx hash:", txHash.Hex())
}

func getSignerFn(
	signerType SignerType,
	chainId *big.Int,
	from common.Address,
	pk *ecdsa.PrivateKey,
) bind.SignerFn {
	return func(signer types.Signer, address common.Address, tx *types.Transaction) (*types.Transaction, error) {
		if address != from {
			err := errors.Errorf("not authorized to sign with %s", address.Hex())
			return nil, err
		}

		// default signer is Homestead, but can be overidden
		if signerType == SignerEIP155 {
			signer = types.NewEIP155Signer(chainId)
		}

		txHash := signer.Hash(tx)
		log.Printf("signer: %T", signer)
		log.Println("signer obtained tx hash:", txHash.Hex())

		signature, err := crypto.Sign(txHash.Bytes(), pk)
		if err != nil {
			return nil, err
		}

		return tx.WithSignature(signer, signature)
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
		signedTx, err := opts.Signer(types.HomesteadSigner{}, opts.From, rawTx)
		if err != nil {
			return nil, err
		}

		txHash, err := ec.SendTransactionWithRet(opts.Context, signedTx)
		if err != nil {
			return nil, err
		}

		*txHashOut = txHash
		return signedTx, nil
	}
}
