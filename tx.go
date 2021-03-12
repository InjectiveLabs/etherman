package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

func onTx(cmd *cli.Cmd) {
	bytecodeOnly := cmd.BoolOpt("bytecode", false, "Produce hex-encoded ABI-packed params bytecode only. Do not interact with RPC.")
	contractAddress := cmd.StringArg("ADDRESS", "", "Contract address to interact with.")
	methodName := cmd.StringArg("METHOD", "", "Contract method to transact.")
	methodArgs := cmd.StringsArg("ARGS", []string{}, "Method transaction arguments. Will be ABI-encoded.")
	await := cmd.BoolOpt("await", true, "Await transaction confirmation from the RPC.")

	cmd.Spec = "[--await] ADDRESS METHOD [ARGS...]"

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
			deployer.OptionEnableCoverage(*coverage),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		fromAddress, privateKey := getFromAndPk(*fromPrivkey)
		txOpts := deployer.ContractTxOpts{
			From:         fromAddress,
			FromPk:       privateKey,
			SolSource:    *solSource,
			ContractName: *contractName,
			Contract:     common.HexToAddress(*contractAddress),
			BytecodeOnly: *bytecodeOnly,
			Await:        *await,
		}
		if *coverage {
			txOpts.CoverageAgent = deployer.NewCoverageDataCollector(deployer.CoverageModeDefault)
		}

		log.Debugln("sending from", fromAddress.Hex())
		log.Debugln("target contract", txOpts.Contract.Hex())

		txHash, abiPackedArgs, err := d.Tx(
			context.Background(),
			txOpts,
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
			log.Fatalln(err)
		}

		if *bytecodeOnly {
			fmt.Println(hex.EncodeToString(abiPackedArgs))
			return
		}

		if !*await {
			log.WithField("contract", txOpts.Contract.Hex()).Infoln("sent tx", txHash.Hex())
		}

		fmt.Println(txHash.Hex())
	}
}
