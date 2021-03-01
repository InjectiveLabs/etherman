package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func onCall(cmd *cli.Cmd) {
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	methodName := cmd.StringArg("METHOD", "", "Contract method to transact.")
	methodArgs := cmd.StringsArg("ARGS", []string{}, "Method transaction arguments. Will be ABI-encoded.")
	fromAddress := cmd.StringOpt("from", "0x0000000000000000000000000000000000000000", "Estimate transaction using specified from address.")

	cmd.Spec = "[--from] ADDRESS METHOD [ARGS...]"

	cmd.Action = func() {
		solc := getCompiler()

		solSourceFullPath, _ := filepath.Abs(*solSource)
		contract := getCompiledContract(solc, *contractName, solSourceFullPath, true)
		if contract == nil {
			log.Fatalln("contract compilation failed, check logs")
		}
		contract.Address = common.HexToAddress(*contractAddress)
		log.Println("target contract", contract.Address.Hex())
		log.Println("using from address", common.HexToAddress(*fromAddress).Hex())

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

		callCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancelFn()

		callOpts := &bind.CallOpts{
			From:    common.HexToAddress(*fromAddress),
			Context: callCtx,
		}

		var res []interface{}
		if err = boundContract.Call(callOpts, &res, *methodName, mappedArgs...); err != nil {
			log.WithError(err).Fatalln("failed to call contract method")
			return
		}

		v, _ := json.MarshalIndent(res, "", "\t")
		fmt.Println(string(v))
	}
}
