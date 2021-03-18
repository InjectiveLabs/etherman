package main

import (
	"context"
	"fmt"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
)

func onBuild(cmd *cli.Cmd) {
	standardJSON := cmd.BoolOpt("j standard-json", false, "Output standard JSON for use in --standard-json of solc, also Etherscan verification")

	cmd.Action = func() {
		d, err := deployer.New(
			// only options applicable to build
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
			deployer.OptionSolcAllowedPaths(*solAllowedPaths),
			deployer.OptionEnableCoverage(*coverage),
		)
		if err != nil {
			log.WithError(err).Fatalln("failed to init deployer")
		}

		contract, err := d.Build(
			context.Background(),
			*solSource,
			*contractName,
		)
		if err != nil {
			log.Fatalln(err)
		}

		if *standardJSON {
			out, err := collectPathsToStandardJSON(
				contract.AllPaths,
				true,
				200,
				EVMVersionIstanbul,
			)
			if err != nil {
				log.Fatalln(err)
			}

			fmt.Println(string(out))
			return
		}

		fmt.Println(contract.Bin)
	}
}
