package deployer

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type AbiMethodInputMapperFunc func(args abi.Arguments) []interface{}

type ContractCallOpts struct {
	EVMEndpoint  string
	From         common.Address
	SolSource    string
	ContractName string
	Contract     common.Address
}

func (d *deployer) Call(
	ctx context.Context,
	callOpts ContractCallOpts,
	methodName string,
	methodInputMapper AbiMethodInputMapperFunc,
) (output []interface{}, outputAbi abi.Arguments, err error) {
	solSourceFullPath, _ := filepath.Abs(callOpts.SolSource)
	contract := d.getCompiledContract(callOpts.ContractName, solSourceFullPath, true)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return nil, nil, ErrCompilationFailed
	}
	contract.Address = callOpts.Contract

	dialCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	var client *Client
	rc, err := rpc.DialContext(dialCtx, callOpts.EVMEndpoint)
	if err != nil {
		log.WithError(err).Errorln("failed to dial EVM RPC endpoint")
		return nil, nil, ErrEndpointUnreachable
	} else {
		client = NewClient(rc)
	}

	chainCtx, cancelFn := context.WithTimeout(context.Background(), d.options.RPCTimeout)
	defer cancelFn()

	chainId, err := client.ChainID(chainCtx)
	if err != nil {
		log.WithError(err).Errorln("failed get valid chain ID")
		return nil, nil, err
	} else {
		log.Infoln("got chainID", chainId.String())
	}

	boundContract, err := BindContract(client.Client, contract)
	if err != nil {
		log.WithField("contract", callOpts.ContractName).WithError(err).Errorln("failed to bind contract")
		return nil, nil, err
	}

	method, ok := boundContract.ABI().Methods[methodName]
	if !ok {
		log.WithField("contract", callOpts.ContractName).Errorf("method not found: %s", methodName)
		return nil, nil, err
	}

	mappedArgs := methodInputMapper(method.Inputs)

	callCtx, cancelFn := context.WithTimeout(context.Background(), d.options.CallTimeout)
	defer cancelFn()

	ethCallOpts := &bind.CallOpts{
		From:    callOpts.From,
		Context: callCtx,
	}

	if err := boundContract.Call(ethCallOpts, &output, methodName, mappedArgs...); err != nil {
		err = errors.Wrap(err, "failed to call contract method")
		return nil, nil, err
	}

	return output, method.Outputs, nil
}
