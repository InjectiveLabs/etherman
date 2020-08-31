package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func onLogs(cmd *cli.Cmd) {
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	txHash := cmd.StringArg("TX_HASH", "", "Transaction hash to find receipt.")
	eventName := cmd.StringArg("EVENT_NAME", "", "Contract event to find in the logs.")

	cmd.Spec = "ADDRESS TX_HASH EVENT_NAME"

	cmd.Action = func() {
		solc := getCompiler()

		solSourceFullPath, _ := filepath.Abs(*solSource)
		contract := getCompiledContract(solc, *contractName, solSourceFullPath, true)
		contract.Address = common.HexToAddress(*contractAddress)

		dialCtx, cancelFn := context.WithTimeout(context.Background(), defaultRPCTimeout)
		defer cancelFn()

		var client *Client
		rc, err := rpc.DialContext(dialCtx, *evmEndpoint)
		if err != nil {
			log.WithError(err).Fatal("failed to dial EVM RPC endpoint")
		} else {
			client = NewClient(rc)
		}

		boundContract, err := BindContract(client.Client, contract)
		if err != nil {
			log.WithField("contract", *contractName).WithError(err).Fatal("failed to bind contract")
		}

		if _, ok := boundContract.ABI().Events[*eventName]; !ok {
			log.WithField("contract", *contractName).Fatalf("event not found: %s", *eventName)
		}

		callCtx, cancelFn := context.WithTimeout(context.Background(), defaultCallTimeout)
		defer cancelFn()

		callLog := log.WithField("txHash", *txHash)
		receipt, err := client.TransactionReceipt(callCtx, common.HexToHash(*txHash))
		if err != nil {
			if err == ethereum.NotFound {
				callLog.Fatalln("transaction not found")
			}

			callLog.WithError(err).Fatalln("failed to get transaction receipt")
			return
		} else if receipt.Status != 1 {
			callLog.Fatalln("transaction reverted without logs")
			return
		}

		events := make([]map[string]interface{}, 0, len(receipt.Logs))
		for _, ethLog := range receipt.Logs {
			out := make(map[string]interface{})
			if err := boundContract.UnpackLogIntoMap(out, *eventName, *ethLog); err == nil {
				events = append(events, out)
			} else {
				log.WithError(err).Warningln("unable to unmarshal log")
			}
		}

		cmdOut, _ := json.MarshalIndent(events, "", "\t")
		fmt.Println(string(cmdOut))
	}
}
