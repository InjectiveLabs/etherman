package main

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

var app = cli.App("evm-deploy-contract", "Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x or later.")

func main() {
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

func getFromAndPk(pkHex string) (common.Address, *ecdsa.PrivateKey) {
	if len(pkHex) == 0 {
		log.Fatal("private key not specified, use -P or --privkey")
	} else {
		pkHex = strings.TrimPrefix(pkHex, "0x")
	}

	privateKey, err := crypto.HexToECDSA(pkHex)
	if err != nil {
		log.WithError(err).Fatal("failed to convert privkey from hex to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	return fromAddress, privateKey
}
