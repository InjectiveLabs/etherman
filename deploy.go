package main

import (
	"context"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi"
	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
)

func onDeploy(cmd *cli.Cmd) {
	bytecodeOnly := cmd.BoolOpt("bytecode", false, "Produce hex-encoded contract bytecode only. Do not interact with RPC.")
	await := cmd.BoolOpt("await", true, "Await transaction confirmation from the RPC.")
	contractArgs := cmd.StringsArg("ARGS", []string{}, "Contract constructor's arguments. Will be ABI-encoded.")

	cmd.Spec = "[--bytecode | --await] [ARGS...]"

	cmd.Action = func() {
		var gasPriceOpt *big.Int
		if gasPriceSet {
			gasPriceOpt = new(big.Int).SetUint64(uint64(*gasPrice))
		}

		d, err := deployer.New(
			// only options applicable to tx
			deployer.OptionEVMRPCEndpoint(*evmEndpoint),
			deployer.OptionGasPrice(gasPriceOpt),
			deployer.OptionGasLimit(uint64(*gasLimit)),
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		fromAddress, privateKey := getFromAndPk(*fromPrivkey)
		log.Debugln("sending from", fromAddress.Hex())

		deployOpts := deployer.ContractDeployOpts{
			From:         fromAddress,
			FromPk:       privateKey,
			SolSource:    *solSource,
			ContractName: *contractName,
		}
		txHash, contract, err := d.Deploy(
			context.Background(),
			deployOpts,
			func(args abi.Arguments) []interface{} {
				mappedArgs, err := mapStringArgs(args, *contractArgs)
				if err != nil {
					log.WithError(err).Fatalln("failed to map constructor args")
					return nil
				}

				return mappedArgs
			},
			*bytecodeOnly,
			*await,
		)
		if err != nil {
			os.Exit(1)
		}

		if *bytecodeOnly {
			fmt.Println(contract.Bin)
			return
		}

		if !*await {
			log.WithField("txHash", txHash.Hex()).Infoln("contract address", contract.Address.Hex())
		}

		fmt.Println(contract.Address.Hex())
	}
}
