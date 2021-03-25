package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/InjectiveLabs/etherman/deployer"
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
			deployer.OptionRPCTimeout(duration(*rpcTimeout, defaultRPCTimeout)),
			deployer.OptionCallTimeout(duration(*callTimeout, defaultCallTimeout)),
			deployer.OptionTxTimeout(duration(*txTimeout, defaultTxTimeout)),

			// only options applicable to call
			deployer.OptionEVMRPCEndpoint(*evmEndpoint),
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
			deployer.OptionSolcAllowedPaths(*solAllowedPaths),
			deployer.OptionEnableCoverage(*coverage),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		logsOpts := deployer.ContractLogsOpts{
			SolSource:    *solSource,
			ContractName: *contractName,
			Contract:     common.HexToAddress(*contractAddress),
		}
		if *coverage {
			logsOpts.CoverageAgent = deployer.NewCoverageDataCollector(deployer.CoverageModeDefault)
		}

		log.Debugln("target contract", logsOpts.Contract.Hex())
		log.Debugln("target tx", *txHash)
		log.Debugln("target event name", *eventName)

		events, err := d.Logs(
			context.Background(),
			logsOpts,
			common.HexToHash(*txHash),
			*eventName,
			nil,
		)
		if err != nil {
			log.Fatalln(err)
		}

		cmdOut, _ := json.MarshalIndent(events, "", "\t")
		fmt.Println(string(cmdOut))

		if *coverage {
			logsOpts.CoverageAgent.ReportHTML(nil, *contractName)
		}
	}
}
