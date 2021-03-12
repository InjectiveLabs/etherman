package deployer

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"
)

var (
	ErrNoCoverage           = errors.New("coverage not enabled")
	ErrNoCoverageInContract = errors.New("coverage not compiled into the contract")
)

func (d *deployer) GetCoverageEventInfo(
	ctx context.Context,
	from common.Address,
	contractName string,
	contractAddress common.Address,
) (eventName string, eventABI abi.Event, err error) {
	onlyError := func(err error) (string, abi.Event, error) {
		return "", abi.Event{}, err
	}

	if !d.options.EnableCoverage {
		return onlyError(ErrNoCoverage)
	}

	methodName, methodCalldata, methodABI, err := coverageDefinitionIDCalldata(contractName)
	if err != nil {
		return onlyError(err)
	}

	callMsg := ethereum.CallMsg{
		From: from,
		To:   &contractAddress,
		Data: methodCalldata,
	}

	client, err := d.Backend()
	if err != nil {
		return onlyError(err)
	}

	res, err := client.CallContract(ctx, callMsg, nil)
	if err != nil {
		log.WithError(err).Errorln("failed to get Coverage Definition ID from the contract, was it deployed with coverage enabled?")
		return onlyError(err)
	}

	var coverageDefinitionID uint64
	if values, err := methodABI.Unpack(methodName, res); err != nil {
		log.WithError(err).Errorf("failed to unpack ABI response of %s from the contract", methodName)
		return onlyError(ErrNoCoverageInContract)
	} else {
		if err := methodABI.Methods[methodName].Outputs.Copy(&coverageDefinitionID, values); err != nil {
			log.WithError(err).Errorf("failed to parse response of %s from the contract", methodName)
			return onlyError(ErrNoCoverageInContract)
		}
	}

	if coverageDefinitionID == 0 {
		log.WithError(err).Errorln("Got Coverage Definition ID as zero, does the contract have coverage enabled?")
		return onlyError(ErrNoCoverageInContract)
	}

	eventName, coverageEventABI := NewCoverageMarkerEvent(coverageDefinitionID)
	return eventName, coverageEventABI.Events[eventName], nil
}

type coverageMarkerEventOpts struct {
	EventDefinitionID uint64
}

func NewCoverageMarkerEvent(definitionID uint64) (name string, eventABI abi.ABI) {
	buf := new(bytes.Buffer)

	if err := coverageMarkerEventTemplate.Execute(buf, coverageMarkerEventOpts{
		EventDefinitionID: definitionID,
	}); err != nil {
		panic(err)
	}

	eventABI, _ = abi.JSON(buf)
	name = fmt.Sprintf("___coverage_%d", definitionID)

	return name, eventABI
}

type coverageDefinitionIDABIOpts struct {
	ContractName string
}

func coverageDefinitionIDCalldata(contractName string) (string, []byte, abi.ABI, error) {
	buf := new(bytes.Buffer)

	if err := coverageDefinitionIDABITemplate.Execute(buf, coverageDefinitionIDABIOpts{
		ContractName: contractName,
	}); err != nil {
		panic(err)
	}

	methodABI, err := abi.JSON(buf)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse coverageDefinitionIDABI with contract name %s", contractName)
		return "", nil, abi.ABI{}, err
	}

	methodName := fmt.Sprintf("___coverage_id_%s", contractName)
	calldata, _ := methodABI.Pack(methodName)
	return methodName, calldata, methodABI, nil
}

var coverageMarkerEventTemplate = template.Must(template.New("coverageMarkerEvent").Parse(`[{
	"anonymous": false,
	"inputs": [
		{
			"indexed": false,
			"internalType": "uint64",
			"name": "start",
			"type": "uint64"
		},
		{
			"indexed": false,
			"internalType": "uint64",
			"name": "end",
			"type": "uint64"
		},
		{
			"indexed": false,
			"internalType": "uint64",
			"name": "file",
			"type": "uint64"
		}
	],
	"name": "___coverage_{{.EventDefinitionID}}",
	"type": "event"
}]`))

var coverageDefinitionIDABITemplate = template.Must(template.New("coverageDefinitionIDABI").Parse(`[{
	"inputs": [],
	"name": "___coverage_id_{{.ContractName}}",
	"outputs": [
		{
			"internalType": "uint64",
			"name": "",
			"type": "uint64"
		}
	],
	"stateMutability": "view",
	"type": "function"
}]`))
