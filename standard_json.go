package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

func collectPathsToStandardJSON(
	paths []string,
	optimizer bool,
	optimizerRuns int,
	evmVersion EVMVersion,
) ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		err = errors.Wrap(err, "unable to get current workdir")
		return nil, err
	}

	result := StandardJSONInput{
		Language: "Solidity",
		Sources:  make(map[string]ContractContent, len(paths)),
	}

	result.Settings.Remappings = make([]string, 0)
	result.Settings.Optimizer.Enabled = optimizer
	result.Settings.Optimizer.Runs = optimizerRuns
	result.Settings.EvmVersion = evmVersion

	for _, srcPath := range paths {
		solContent, err := ioutil.ReadFile(srcPath)
		if err != nil {
			err = errors.Wrapf(err, "failed to collect Solidity file %s", srcPath)
			return nil, err
		}

		srcPath = strings.Replace(srcPath, cwd, ".", 1)

		result.Sources[srcPath] = ContractContent{
			Keccak256: crypto.Keccak256Hash(solContent).Hex(),
			Content:   string(solContent),
		}
	}

	return json.MarshalIndent(result, "", "\t")
}

type EVMVersion string

const (
	EVMVersionTangerineWhistle EVMVersion = "tangerineWhistle"
	EVMVersionSpuriousDragon   EVMVersion = "spuriousDragon"
	EVMVersionByzantium        EVMVersion = "byzantium"
	EVMVersionConstantinople   EVMVersion = "constantinople"
	EVMVersionPetersburg       EVMVersion = "petersburg"
	EVMVersionIstanbul         EVMVersion = "istanbul"
	EVMVersionBerlin           EVMVersion = "berlin"
)

type ContractContent struct {
	Keccak256 string `json:"keccak256"`
	Content   string `json:"content"`
}

type StandardJSONInput struct {
	Language string                     `json:"language"`
	Sources  map[string]ContractContent `json:"sources"`
	Settings struct {
		Remappings []string `json:"remappings"`

		Optimizer struct {
			Enabled bool `json:"enabled"`
			Runs    int  `json:"runs"`
		} `json:"optimizer"`

		EvmVersion EVMVersion `json:"evmVersion"`
	} `json:"settings"`
}
