package main

import (
	"time"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

const (
	defaultRPCTimeout  = 10 * time.Second
	defaultTxTimeout   = 30 * time.Second
	defaultCallTimeout = 10 * time.Second
)

var (
	solcPathSet bool
	solcPath    *string

	contractName    *string
	solSource       *string
	solAllowedPaths *[]string
	evmEndpoint     *string

	rpcTimeout  *string
	txTimeout   *string
	callTimeout *string

	gasPrice      *int
	gasLimit      *int
	buildCacheDir *string
	noCache       *bool
	coverage      *bool
	logLevel      *string
)

func readGlobalOptions(
	solcPathSet *bool,
	solcPath **string,

	contractName **string,
	solSource **string,
	solAllowedPaths **[]string,
	evmEndpoint **string,

	rpcTimeout **string,
	txTimeout **string,
	callTimeout **string,

	gasPrice **int,
	gasLimit **int,
	buildCacheDir **string,
	noCache **bool,
	coverage **bool,
	logLevel **string,
) {
	*solcPath = app.String(cli.StringOpt{
		Name:      "solc-path",
		Desc:      "Set path solc executable. Found using 'which' otherwise",
		EnvVar:    "DEPLOYER_SOLC_PATH",
		Value:     "",
		SetByUser: solcPathSet,
	})

	*contractName = app.String(cli.StringOpt{
		Name:   "N name",
		Desc:   "Specify contract name to use.",
		EnvVar: "DEPLOYER_CONTRACT_NAME",
		Value:  "Counter",
	})

	*solSource = app.String(cli.StringOpt{
		Name:   "S source",
		Desc:   "Set path for .sol source file of the contract.",
		EnvVar: "DEPLOYER_SOL_SOURCE_FILE",
		Value:  "contracts/Counter.sol",
	})

	*solAllowedPaths = app.Strings(cli.StringsOpt{
		Name:   "allowed-paths",
		Desc:   "Specify allowed paths to Solc compiler, allows to include contracts from outside workdir",
		EnvVar: "DEPLOYER_SOL_ALLOWED_PATHS",
		Value:  []string{},
	})

	*evmEndpoint = app.String(cli.StringOpt{
		Name:   "E endpoint",
		Desc:   "Specify the JSON-RPC endpoint for accessing Ethereum node",
		EnvVar: "DEPLOYER_RPC_URI",
		Value:  "http://localhost:8545",
	})

	*rpcTimeout = app.String(cli.StringOpt{
		Name:   "rpc-timeout",
		Desc:   "Specify overall timeout of an RPC request (e.g. 15s).",
		EnvVar: "DEPLOYER_RPC_TIMEOUT",
		Value:  "10s",
	})

	*txTimeout = app.String(cli.StringOpt{
		Name:   "tx-timeout",
		Desc:   "Specify overall timeout of a Transaction, including confirmation await (e.g. 50s).",
		EnvVar: "DEPLOYER_TX_TIMEOUT",
		Value:  "30s",
	})

	*callTimeout = app.String(cli.StringOpt{
		Name:   "call-timeout",
		Desc:   "Specify overall timeout of an EVM call (e.g. 15s).",
		EnvVar: "DEPLOYER_CALL_TIMEOUT",
		Value:  "10s",
	})

	*gasPrice = app.Int(cli.IntOpt{
		Name:   "G gas-price",
		Desc:   "Override estimated gas price with this option.",
		EnvVar: "DEPLOYER_TX_GAS_PRICE",
		Value:  -1, // estimate
	})

	*gasLimit = app.Int(cli.IntOpt{
		Name:   "L gas-limit",
		Desc:   "Set the maximum gas for tx.",
		EnvVar: "DEPLOYER_TX_GAS_LIMIT",
		Value:  5000000,
	})

	*buildCacheDir = app.String(cli.StringOpt{
		Name:   "cache-dir",
		Desc:   "Set cache dir for build artifacts.",
		EnvVar: "DEPLOYER_CACHE_DIR",
		Value:  "build/",
	})

	*noCache = app.Bool(cli.BoolOpt{
		Name:   "no-cache",
		Desc:   "Disables build cache completely.",
		EnvVar: "DEPLOYER_DISABLE_CACHE",
		Value:  false,
	})

	*coverage = app.Bool(cli.BoolOpt{
		Name:   "cover",
		Desc:   "Enables code coverage orchestration",
		EnvVar: "DEPLOYER_ENABLE_COVERAGE",
		Value:  false,
	})

	*logLevel = app.String(cli.StringOpt{
		Name:   "l log-level",
		Desc:   "Available levels: error, warn, info, debug.",
		EnvVar: "DEPLOYER_LOG_LEVEL",
		Value:  "info",
	})
}

func toLogLevel(s string) log.Level {
	switch s {
	case "1", "error":
		return log.ErrorLevel
	case "2", "warn":
		return log.WarnLevel
	case "3", "info":
		return log.InfoLevel
	case "4", "debug":
		return log.DebugLevel
	default:
		return log.FatalLevel
	}
}

func duration(s string, defaults time.Duration) time.Duration {
	dur, err := time.ParseDuration(s)
	if err != nil {
		dur = defaults
	}
	return dur
}
