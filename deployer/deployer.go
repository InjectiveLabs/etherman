package deployer

import (
	"context"
	"math/big"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/etherman/sol"
)

var (
	ErrCompilerNotFound = errors.New("unable to locate Solidity compiler")
)

func init() {
	log.DefaultLogger.SetLevel(log.InfoLevel)
}

type option func(o *options) error

func New(opts ...option) (Deployer, error) {
	d := &deployer{
		options: defaultOptions(),
	}

	for _, o := range opts {
		if err := o(d.options); err != nil {
			err = errors.Wrap(err, "error in deployer option")
			return nil, err
		}
	}

	if d.options.SolcPathSet {
		solc, err := sol.NewSolCompiler(d.options.SolcPath)
		if err != nil {
			log.WithField("path", d.options.SolcPath).WithError(err).Errorln("failed to find solc compiler at path")
			return nil, ErrCompilerNotFound
		}

		d.compiler = solc
	} else {
		solcPathFound, err := sol.WhichSolc()
		if err != nil {
			log.WithError(err).Errorln("failed to find solc compiler")
			return nil, ErrCompilerNotFound
		}

		solc, err := sol.NewSolCompiler(solcPathFound)
		if err != nil {
			log.WithField("path", solcPathFound).WithError(err).Errorln("failed to find solc compiler at path")
			return nil, ErrCompilerNotFound
		}

		d.compiler = solc
	}

	for i := range d.options.SolcAllowedPaths {
		abs, err := filepath.Abs(d.options.SolcAllowedPaths[i])
		if err == nil {
			d.options.SolcAllowedPaths[i] = abs
		}
	}

	d.compiler.SetAllowPaths(d.options.SolcAllowedPaths)

	return d, nil
}

type Deployer interface {
	Backend() (*Client, error)

	Build(
		ctx context.Context,
		solSource string,
		contractName string,
	) (*sol.Contract, error)

	Deploy(
		ctx context.Context,
		deployOpts ContractDeployOpts,
		constructorInputMapper AbiMethodInputMapperFunc,
	) (txHash common.Hash, contract *sol.Contract, err error)

	Tx(
		ctx context.Context,
		txOpts ContractTxOpts,
		methodName string,
		methodInputMapper AbiMethodInputMapperFunc,
	) (txHash common.Hash, abiPackedArgs []byte, err error)

	Call(
		ctx context.Context,
		callOpts ContractCallOpts,
		methodName string,
		methodInputMapper AbiMethodInputMapperFunc,
	) (output []interface{}, outputAbi abi.Arguments, err error)

	Logs(
		ctx context.Context,
		logsOpts ContractLogsOpts,
		txHash common.Hash,
		eventName string,
		eventUnpacker ContractLogUnpackFunc,
	) (events []interface{}, err error)
}

type deployer struct {
	options  *options
	compiler sol.Compiler
	client   *Client

	initClientOnce sync.Once
}

type options struct {
	RPCTimeout  time.Duration
	TxTimeout   time.Duration
	CallTimeout time.Duration

	SignerType     SignerType
	GasPrice       *big.Int
	GasLimit       uint64
	EVMRPCEndpoint string

	NoCache          bool
	BuildCacheDir    string
	SolcPath         string
	SolcPathSet      bool
	EnableCoverage   bool
	SolcAllowedPaths []string
}

func defaultOptions() *options {
	return &options{
		RPCTimeout:  10 * time.Second,
		TxTimeout:   30 * time.Second,
		CallTimeout: 10 * time.Second,

		SignerType:     SignerEIP155,
		GasPrice:       new(big.Int),
		GasLimit:       1000000,
		EVMRPCEndpoint: "http://localhost:8545",

		NoCache:        false,
		BuildCacheDir:  "build",
		EnableCoverage: false,
	}
}

func OptionRPCTimeout(dur time.Duration) option {
	return func(o *options) error {
		if dur > time.Millisecond {
			o.RPCTimeout = dur
		}

		return nil
	}
}

func OptionTxTimeout(dur time.Duration) option {
	return func(o *options) error {
		if dur > time.Millisecond {
			o.TxTimeout = dur
		}

		return nil
	}
}

func OptionCallTimeout(dur time.Duration) option {
	return func(o *options) error {
		if dur > time.Millisecond {
			o.CallTimeout = dur
		}

		return nil
	}
}

func OptionSignerType(signerType SignerType) option {
	return func(o *options) error {
		if len(signerType) == 0 {
			return errors.New("signer type not specified")
		}

		o.SignerType = signerType
		return nil
	}
}

func OptionGasPrice(price *big.Int) option {
	return func(o *options) error {
		if price != nil && price.String() == "-1" {
			o.GasPrice = nil
			return nil
		}

		// nil price means we will estimate
		o.GasPrice = price

		return nil
	}
}

func OptionGasLimit(gasLimit uint64) option {
	return func(o *options) error {
		if gasLimit < 21000 {
			return errors.New("gas limit too low")
		}

		o.GasLimit = gasLimit
		return nil
	}
}

func OptionNoCache(noCache bool) option {
	return func(o *options) error {
		o.NoCache = noCache
		return nil
	}
}

func OptionBuildCacheDir(dir string) option {
	return func(o *options) error {
		if len(dir) == 0 {
			return errors.New("empty build cache dir provided")
		}

		o.BuildCacheDir = dir
		return nil
	}
}

func OptionEVMRPCEndpoint(uri string) option {
	return func(o *options) error {
		_, err := url.ParseRequestURI(uri)
		if err != nil {
			return errors.Wrap(err, "failed to parse EVM RPC Endpoint")
		}

		o.EVMRPCEndpoint = uri
		return nil
	}
}

func OptionSolcPath(dir string) option {
	return func(o *options) error {
		if len(dir) == 0 {
			o.SolcPathSet = false
		} else {
			o.SolcPathSet = true
		}

		o.SolcPath = dir
		return nil
	}
}

func OptionEnableCoverage(enabled bool) option {
	return func(o *options) error {
		o.EnableCoverage = enabled
		return nil
	}
}

func OptionSolcAllowedPaths(allowedPaths []string) option {
	return func(o *options) error {
		o.SolcAllowedPaths = allowedPaths
		return nil
	}
}
