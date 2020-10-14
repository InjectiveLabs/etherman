package main

import (
	"fmt"
	"os"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

var app = cli.App("evm-deploy-contract", "Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x")

func main() {
	app.Action = func() {
		fmt.Println("You should use either deploy, tx or logs command. See --help for more info.")
	}

	app.Command("build", "Builds given contract and cached build artefacts. Optional step.", onBuild)
	app.Command("deploy", "Deploys given contract on the EVM chain. Caches build artefacts.", onDeploy)
	app.Command("tx", "Creates a transaction for particular contract method. Uses build cache.", onTx)
	app.Command("logs", "Loads logs of a particular event from contract.", onLogs)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
