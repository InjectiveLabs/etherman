package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

func onCall(cmd *cli.Cmd) {
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	methodName := cmd.StringArg("METHOD", "", "Contract method to transact.")
	methodArgs := cmd.StringsArg("ARGS", []string{}, "Method transaction arguments. Will be ABI-encoded.")
	fromAddress := cmd.StringOpt("from", "0x0000000000000000000000000000000000000000", "Estimate transaction using specified from address.")

	cmd.Spec = "[--from] ADDRESS METHOD [ARGS...]"

	cmd.Action = func() {
		d, err := deployer.New(
			// only options applicable to call
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		callOpts := deployer.ContractCallOpts{
			EVMEndpoint:  *evmEndpoint,
			From:         common.HexToAddress(*fromAddress),
			SolSource:    *solSource,
			ContractName: *contractName,
			Contract:     common.HexToAddress(*contractAddress),
		}

		log.Debugln("target contract", callOpts.Contract.Hex())
		log.Debugln("using from address", callOpts.From.Hex())

		output, _, err := d.Call(
			context.Background(),
			callOpts,
			*methodName,
			func(args abi.Arguments) []interface{} {
				mappedArgs, err := mapStringArgs(args, *methodArgs)
				if err != nil {
					log.WithError(err).Fatalln("failed to map method args")
					return nil
				}

				return mappedArgs
			},
		)
		if err != nil {
			os.Exit(1)
		}

		v, _ := json.MarshalIndent(output, "", "\t")
		fmt.Println(string(v))
	}
}
