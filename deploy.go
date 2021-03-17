package main

import (
	"context"
	"fmt"
	"math/big"

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
		d, err := deployer.New(
			deployer.OptionRPCTimeout(duration(*rpcTimeout, defaultRPCTimeout)),
			deployer.OptionCallTimeout(duration(*callTimeout, defaultCallTimeout)),
			deployer.OptionTxTimeout(duration(*txTimeout, defaultTxTimeout)),

			// only options applicable to tx
			deployer.OptionEVMRPCEndpoint(*evmEndpoint),
			deployer.OptionGasPrice(big.NewInt(int64(*gasPrice))),
			deployer.OptionGasLimit(uint64(*gasLimit)),
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
			deployer.OptionSolcAllowedPaths(*solAllowedPaths),
			deployer.OptionEnableCoverage(*coverage),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		client, err := d.Backend()
		if err != nil {
			log.Fatalln(err)
		}

		chainCtx, cancelFn := context.WithTimeout(context.Background(), duration(*rpcTimeout, defaultRPCTimeout))
		defer cancelFn()

		chainID, err := client.ChainID(chainCtx)
		if err != nil {
			log.WithError(err).Fatalln("failed get valid chain ID")
		}

		fromAddress, signerFn, err := initEthereumAccountsManager(
			chainID.Uint64(),
			keystoreDir,
			from,
			fromPassphrase,
			fromPrivKey,
			useLedger,
		)
		if err != nil {
			log.WithError(err).Fatalln("failed init SignerFn")
		}

		log.Debugln("sending from", fromAddress.Hex())

		deployOpts := deployer.ContractDeployOpts{
			From:         fromAddress,
			SignerFn:     signerFn,
			SolSource:    *solSource,
			ContractName: *contractName,
			BytecodeOnly: *bytecodeOnly,
			Await:        *await,
		}
		if *coverage {
			deployOpts.CoverageAgent = deployer.NewCoverageDataCollector(deployer.CoverageModeDefault)
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
		)
		if err != nil {
			log.Fatalln(err)
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
