package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/common"
)

func onLogs(cmd *cli.Cmd) {
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	txHash := cmd.StringArg("TX_HASH", "", "Transaction hash to find receipt.")
	eventName := cmd.StringArg("EVENT_NAME", "", "Contract event to find in the logs.")

	cmd.Spec = "ADDRESS TX_HASH EVENT_NAME"

	cmd.Action = func() {
		d, err := deployer.New(
			// only options applicable to call
			deployer.OptionEVMRPCEndpoint(*evmEndpoint),
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		logsOpts := deployer.ContractLogsOpts{
			SolSource:    *solSource,
			ContractName: *contractName,
			Contract:     common.HexToAddress(*contractAddress),
		}

		log.Debugln("target contract", logsOpts.Contract.Hex())
		log.Debugln("target tx", *txHash)
		log.Debugln("target event name", *eventName)

		events, err := d.Logs(
			context.Background(),
			logsOpts,
			common.HexToHash(*txHash),
			*eventName,
		)
		if err != nil {
			os.Exit(1)
		}

		cmdOut, _ := json.MarshalIndent(events, "", "\t")
		fmt.Println(string(cmdOut))
	}
}
