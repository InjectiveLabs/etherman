package main

import (
	"context"
	"fmt"
	"os"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/evm-deploy-contract/deployer"
)

func onBuild(cmd *cli.Cmd) {
	cmd.Action = func() {
		d, err := deployer.New(
			// only options applicable to build
			deployer.OptionNoCache(*noCache),
			deployer.OptionBuildCacheDir(*buildCacheDir),
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
			os.Exit(1)
		}

		fmt.Println(contract.Bin)
	}
}
