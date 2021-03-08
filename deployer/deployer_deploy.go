package deployer

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"path/filepath"

	"github.com/InjectiveLabs/evm-deploy-contract/sol"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrEndpointUnreachable = errors.New("unable to dial EVM RPC endpoint")
	ErrNoChainID           = errors.New("failed to get valid Chain ID")
	ErrNoNonce             = errors.New("failed to get latest from nonce")
)

type ContractDeployOpts struct {
	From         common.Address
	FromPk       *ecdsa.PrivateKey
	SolSource    string
	ContractName string
	BytecodeOnly bool
	Await        bool
}

func (d *deployer) Deploy(
	ctx context.Context,
	deployOpts ContractDeployOpts,
	constructorInputMapper AbiMethodInputMapperFunc,
) (txHash common.Hash, contract *sol.Contract, err error) {
	solSourceFullPath, _ := filepath.Abs(deployOpts.SolSource)
	contract = d.getCompiledContract(deployOpts.ContractName, solSourceFullPath, false)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return noHash, nil, ErrCompilationFailed
	}

	if !d.options.NoCache {
		cacheLog := log.WithField("path", d.options.BuildCacheDir)
		cache, err := NewBuildCache(d.options.BuildCacheDir)
		if err != nil {
			cacheLog.WithError(err).Warningln("failed to use build cache dir")
		} else if err := cache.StoreContract(solSourceFullPath, contract); err != nil {
			cacheLog.WithError(err).Warningln("failed to store contract code in build cache")
		}
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

	nonce, err := client.PendingNonceAt(nonceCtx, deployOpts.From)
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

	signerFn, err := getSignerFn(d.options.SignerType, chainId, deployOpts.From, deployOpts.FromPk)
	if err != nil {
		log.WithError(err).Errorln("failed to get signer function")
		return noHash, nil, err
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

	address, _, err := boundContract.DeployContract(ethTxOpts, mappedArgs...)
	if err != nil {
		log.WithError(err).Errorln("failed to deploy contract")
		return txHash, nil, err
	}
	contract.Address = address

	if deployOpts.Await {
		awaitCtx, cancelFn := context.WithTimeout(context.Background(), d.options.TxTimeout)
		defer cancelFn()

		log.WithField("txHash", txHash.Hex()).Debugln("awaiting contract deployment", address.Hex())

		_, err = awaitTx(awaitCtx, client, txHash)
	}

	return txHash, contract, err
}

var noHash = common.Hash{}
