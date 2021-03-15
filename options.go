package main

import (
	"time"

	cli "github.com/jawher/mow.cli"
)

const (
	defaultRPCTimeout  = 10 * time.Second
	defaultTxTimeout   = 10 * time.Second
	defaultCallTimeout = 10 * time.Second
)

var (
	solcPathSet bool
	solcPath    = app.String(cli.StringOpt{
		Name:      "solc-path",
		Desc:      "Set path solc executable. Found using 'which' otherwise",
		EnvVar:    "DEPLOYER_SOLC_PATH",
		Value:     "",
		SetByUser: &solcPathSet,
	})

	contractName = app.String(cli.StringOpt{
		Name:   "N name",
		Desc:   "Specify contract name to use.",
		EnvVar: "DEPLOYER_CONTRACT_NAME",
		Value:  "Counter",
	})

	solSource = app.String(cli.StringOpt{
		Name:   "S source",
		Desc:   "Set path for .sol source file of the contract.",
		EnvVar: "DEPLOYER_SOL_SOURCE_FILE",
		Value:  "contracts/Counter.sol",
	})

	evmEndpoint = app.String(cli.StringOpt{
		Name:   "E endpoint",
		Desc:   "Specify the JSON-RPC endpoint for accessing Ethereum node",
		EnvVar: "DEPLOYER_RPC_URI",
		Value:  "http://localhost:8545",
	})

	gasPriceSet bool
	gasPrice    = app.Int(cli.IntOpt{
		Name:      "G gas-price",
		Desc:      "Override estimated gas price with this option.",
		EnvVar:    "DEPLOYER_TX_GAS_PRICE",
		Value:     50, // wei
		SetByUser: &gasPriceSet,
	})

	gasLimit = app.Int(cli.IntOpt{
		Name:   "L gas-limit",
		Desc:   "Set the maximum gas for tx.",
		EnvVar: "DEPLOYER_TX_GAS_LIMIT",
		Value:  5000000,
	})

	buildCacheDir = app.String(cli.StringOpt{
		Name:   "cache-dir",
		Desc:   "Set cache dir for build artifacts.",
		EnvVar: "DEPLOYER_CACHE_DIR",
		Value:  "build/",
	})

	noCache = app.Bool(cli.BoolOpt{
		Name:   "no-cache",
		Desc:   "Disables build cache completely.",
		EnvVar: "DEPLOYER_DISABLE_CACHE",
		Value:  false,
	})

	coverage = app.Bool(cli.BoolOpt{
		Name:   "cover",
		Desc:   "Enables code coverage orchestration",
		EnvVar: "DEPLOYER_ENABLE_COVERAGE",
		Value:  false,
	})
)
