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

func onTx(cmd *cli.Cmd) {
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	methodName := cmd.StringArg("METHOD", "", "Contract method to transact.")
	methodArgs := cmd.StringsArg("ARGS", []string{}, "Method transaction arguments. Will be ABI-encoded.")
	await := cmd.BoolOpt("await", true, "Await transaction confirmation from the RPC.")

	cmd.Spec = "[--await] ADDRESS METHOD [ARGS...]"

	cmd.Action = func() {
		solc := getCompiler()

		solSourceFullPath, _ := filepath.Abs(*solSource)
		contract := getCompiledContract(solc, *contractName, solSourceFullPath, true)
		contract.Address = common.HexToAddress(*contractAddress)

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

		method, ok := boundContract.ABI().Methods[*methodName]
		if !ok {
			log.WithField("contract", *contractName).Fatalf("method not found: %s", *methodName)
		}

		mappedArgs, err := mapStringArgs(method.Inputs, *methodArgs)
		if err != nil {
			log.WithError(err).Fatalln("failed to map method args")
		}

		var txHash common.Hash
		boundContract.SetTransact(getTransactFn(client, contract.Address, &txHash))

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

		if _, err = boundContract.Transact(txOpts, *methodName, mappedArgs...); err != nil {
			log.WithError(err).Fatalln("failed to deploy contract")
			return
		}

		fmt.Println(txHash.Hex())

		if *await {
			awaitCtx, cancelFn := context.WithTimeout(context.Background(), defaultTxTimeout)
			defer cancelFn()

			awaitTx(awaitCtx, client, txHash)
		}
	}
}
