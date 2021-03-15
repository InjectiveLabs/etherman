package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

var app = cli.App("evm-deploy-contract", "Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x or later.")

func main() {
	readEnv()

	readGlobalOptions(
		&solcPathSet,
		&solcPath,
		&contractName,
		&solSource,
		&evmEndpoint,
		&gasPriceSet,
		&gasPrice,
		&gasLimit,
		&buildCacheDir,
		&noCache,
		&coverage,
	)

	readEthereumKeyOptions(
		&keystoreDir,
		&from,
		&fromPassphrase,
		&fromPrivKey,
		&useLedger,
	)

	app.Action = func() {
		fmt.Println("You should use either deploy, tx or logs command. See --help for more info.")
	}

	app.Command("build", "Builds given contract and cached build artefacts. Optional step.", onBuild)
	app.Command("deploy", "Deploys given contract on the EVM chain. Caches build artefacts.", onDeploy)
	app.Command("tx", "Creates a transaction for particular contract method. Uses build cache.", onTx)
	app.Command("call", "Calls method of a particular contract. Uses build cache.", onCall)
	app.Command("logs", "Loads logs of a particular event from contract.", onLogs)

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// readEnv is a special utility that reads `.env` file into actual environment variables
// of the current app, similar to `dotenv` Node package.
func readEnv() {
	if envdata, _ := ioutil.ReadFile(".env"); len(envdata) > 0 {
		s := bufio.NewScanner(bytes.NewReader(envdata))
		for s.Scan() {
			parts := strings.Split(s.Text(), "=")
			if len(parts) != 2 {
				continue
			}
			strValue := strings.Trim(parts[1], `"`)
			if err := os.Setenv(parts[0], strValue); err != nil {
				log.WithField("name", parts[0]).WithError(err).Warningln("failed to override ENV variable")
			}
		}
	}
}
