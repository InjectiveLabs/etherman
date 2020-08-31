package main

import (
	"context"
	"fmt"
	"math/big"
	"path/filepath"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func onDeploy(cmd *cli.Cmd) {
	bytecodeOnly := cmd.BoolOpt("bytecode", false, "Produce hex-encoded contract bytecode only. Do not interact with RPC.")
	await := cmd.BoolOpt("await", true, "Await transaction confirmation from the RPC.")
	contractArgs := cmd.StringsArg("ARGS", []string{}, "Contract constructor's arguments. Will be ABI-encoded.")

	cmd.Spec = "[--bytecode | --await] [ARGS...]"

	cmd.Action = func() {
		solc := getCompiler()

		solSourceFullPath, _ := filepath.Abs(*solSource)
		contract := getCompiledContract(solc, *contractName, solSourceFullPath, false)

		if *bytecodeOnly {
			fmt.Println(contract.Bin)
			return
		}

		if !*noCache {
			cacheLog := log.WithField("path", *buildCacheDir)
			cache, err := NewBuildCache(*buildCacheDir)
			if err != nil {
				cacheLog.WithError(err).Warningln("failed to use build cache dir")
			} else if err := cache.StoreContract(solSourceFullPath, contract); err != nil {
				cacheLog.WithError(err).Warningln("failed to store contract code in build cache")
			}
		}

		fromAddress, privateKey := getFromAndPk(*fromPrivkey)
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
			log.Println("got chainID", chainId.String())
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
			log.WithField("contract", *contractName).WithError(err).Fatal("failed to bind contract")
		}

		inputs := boundContract.ABI().Constructor.Inputs
		mappedArgs, err := mapStringArgs(inputs, *contractArgs)
		if err != nil {
			log.WithError(err).Fatalln("failed to map contract args")
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

		address, _, err := boundContract.DeployContract(txOpts, mappedArgs...)
		if err != nil {
			log.WithError(err).Fatalln("failed to deploy contract")
			return
		}

		log.WithField("address", address.Hex()).Infoln("obtained contract address")

		fmt.Println(txHash.Hex())

		if *await {
			awaitCtx, cancelFn := context.WithTimeout(context.Background(), defaultTxTimeout)
			defer cancelFn()

			awaitTx(awaitCtx, client, txHash)
		}
	}
}
