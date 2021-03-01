package deployer

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type ContractTxOpts struct {
	EVMEndpoint  string
	From         common.Address
	FromPk       *ecdsa.PrivateKey
	SolSource    string
	ContractName string
	Contract     common.Address
}

func (d *deployer) Tx(
	ctx context.Context,
	txOpts ContractTxOpts,
	methodName string,
	methodInputMapper AbiMethodInputMapperFunc,
	bytecodeOnly bool,
	await bool,
) (txHash common.Hash, abiPackedArgs []byte, err error) {
	solSourceFullPath, _ := filepath.Abs(txOpts.SolSource)
	contract := d.getCompiledContract(txOpts.ContractName, solSourceFullPath, true)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return noHash, nil, ErrCompilationFailed
	}
	contract.Address = txOpts.Contract

	if !d.options.NoCache {
		cacheLog := log.WithField("path", d.options.BuildCacheDir)
		cache, err := NewBuildCache(d.options.BuildCacheDir)
		if err != nil {
			cacheLog.WithError(err).Warningln("failed to use build cache dir")
		} else if err := cache.StoreContract(solSourceFullPath, contract); err != nil {
			cacheLog.WithError(err).Warningln("failed to store contract code in build cache")
		}
	}

	if bytecodeOnly {
		boundContract, err := BindContract(nil, contract)
		if err != nil {
			log.WithField("contract", txOpts.ContractName).WithError(err).Errorln("failed to bind contract")
			return noHash, nil, err
		}

		mappedArgs := methodInputMapper(boundContract.ABI().Constructor.Inputs)
		abiPackedArgs, err := boundContract.ABI().Constructor.Inputs.PackValues(mappedArgs)
		if err != nil {
			err = errors.Wrap(err, "failed to ABI-encode constructor values")
			return noHash, nil, err
		}

		return noHash, abiPackedArgs, nil
	}

	dialCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	var client *Client
	rc, err := rpc.DialContext(dialCtx, txOpts.EVMEndpoint)
	if err != nil {
		log.WithError(err).Errorln("failed to dial EVM RPC endpoint")
		return noHash, nil, ErrEndpointUnreachable
	} else {
		client = NewClient(rc)
	}

	chainCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	chainId, err := client.ChainID(chainCtx)
	if err != nil {
		log.WithError(err).Errorln("failed get valid chain ID")
		return noHash, nil, ErrNoChainID
	} else {
		log.Println("got chainID", chainId.String())
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

	method, ok := boundContract.ABI().Methods[methodName]
	if !ok {
		log.WithField("contract", txOpts.ContractName).Errorf("method not found: %s", methodName)
		return noHash, nil, err
	}

	mappedArgs := methodInputMapper(method.Inputs)
	boundContract.SetTransact(getTransactFn(client, contract.Address, &txHash))

	txCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	signerFn, err := getSignerFn(d.options.SignerType, chainId, txOpts.From, txOpts.FromPk)
	if err != nil {
		log.WithError(err).Errorln("failed to get signer function")
		return noHash, nil, err
	}

	ethTxOpts := &bind.TransactOpts{
		From:     txOpts.From,
		Nonce:    big.NewInt(int64(nonce)),
		Signer:   signerFn,
		Value:    big.NewInt(0),
		GasPrice: d.options.GasPrice,
		GasLimit: d.options.GasLimit,

		Context: txCtx,
	}

	if _, err = boundContract.Transact(ethTxOpts, methodName, mappedArgs...); err != nil {
		log.WithError(err).Errorln("failed to send transaction")
		return noHash, nil, err
	}

	if await {
		awaitCtx, cancelFn := context.WithTimeout(context.Background(), d.options.TxTimeout)
		defer cancelFn()

		log.WithField("contract", contract.Address.Hex()).Infoln("awaiting tx", txHash.Hex())

		awaitTx(awaitCtx, client, txHash)
	}

	return txHash, nil, nil
}
