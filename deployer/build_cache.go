package deployer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/etherman/sol"
)

var ErrNoCache = errors.New("no cached version")

type BuildCache interface {
	StoreContract(absSolPath string, contract *sol.Contract) error
	LoadContract(absSolPath, contractName string, coverage bool) (contract *sol.Contract, err error)
	Clear() error
}

type BuildCacheEntry struct {
	Timestamp       time.Time       `json:"timestamp"`
	CodeHash        string          `json:"codeHash"`
	AllPaths        []string        `json:"allPaths"`
	ContractName    string          `json:"contractName"`
	CompilerVersion string          `json:"compilerVersion"`
	Coverage        bool            `json:"coverage"`
	Statements      [][]int         `json:"statements"`
	ABI             json.RawMessage `json:"abi"`
	Bin             string          `json:"bin"`
}

type buildCache struct {
	prefix string
}

func NewBuildCache(prefix string) (BuildCache, error) {
	if err := os.MkdirAll(prefix, 0755); err != nil {
		err = errors.Wrap(err, "failed to prepare build cache dir")
		return nil, err
	}

	c := &buildCache{
		prefix: prefix,
	}

	return c, nil
}

func (b *buildCache) StoreContract(absSolPath string, contract *sol.Contract) error {
	hash, err := sha3file(absSolPath)
	if err != nil {
		err = errors.Wrap(err, "failed to hash source")
		return err
	}

	entry := &BuildCacheEntry{
		Timestamp:       time.Now().UTC(),
		CodeHash:        hash,
		AllPaths:        contract.AllPaths,
		ContractName:    contract.Name,
		CompilerVersion: contract.CompilerVersion,
		Coverage:        contract.Coverage,
		Statements:      contract.Statements,
		ABI:             json.RawMessage(contract.ABI),
		Bin:             contract.Bin,
	}

	entryContents, _ := json.MarshalIndent(entry, "", "\t")
	entryFileName := fmt.Sprintf("sol_%s_%s.json", strings.ToLower(contract.Name), hash)
	if contract.Coverage {
		entryFileName = fmt.Sprintf("sol_%s_%s_coverage.json", strings.ToLower(contract.Name), hash)
	}

	err = ioutil.WriteFile(filepath.Join(b.prefix, entryFileName), entryContents, 0655)
	if err != nil {
		err = errors.Wrap(err, "failed write cache entry file")
		return err
	}

	return nil
}

func (b *buildCache) LoadContract(absSolPath, contractName string, coverage bool) (contract *sol.Contract, err error) {
	hash, err := sha3file(absSolPath)
	if err != nil {
		err = errors.Wrap(err, "failed to hash source")
		return nil, err
	}

	entryFileName := fmt.Sprintf("sol_%s_%s.json", strings.ToLower(contractName), hash)
	if coverage {
		entryFileName = fmt.Sprintf("sol_%s_%s_coverage.json", strings.ToLower(contractName), hash)
	}

	entryContents, err := ioutil.ReadFile(filepath.Join(b.prefix, entryFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoCache
		}

		err = errors.Wrap(err, "failed read cache entry file")
		return nil, err
	}

	var entry BuildCacheEntry
	if err := json.Unmarshal(entryContents, &entry); err != nil {
		err = errors.Wrap(err, "failed to unmarshal cache entry")
		return nil, err
	} else if entry.ContractName != contractName {
		err = errors.Wrap(err, "cache entry contract name mismatch")
		return nil, err
	}

	contract = &sol.Contract{
		SourcePath:      absSolPath,
		AllPaths:        entry.AllPaths,
		Name:            entry.ContractName,
		CompilerVersion: entry.CompilerVersion,
		Coverage:        entry.Coverage,
		Statements:      entry.Statements,
		ABI:             []byte(entry.ABI),
		Bin:             entry.Bin,
	}

	return contract, nil
}

func (b *buildCache) Clear() error {
	return filepath.Walk(b.prefix, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if path == b.prefix {
			return nil
		} else if info.IsDir() {
			return nil
		}

		if filepath.Ext(info.Name()) == ".json" {
			if err := os.Remove(path); err != nil {
				log.WithError(err).Warningln("failed to cleanup", path)
			}
		}

		return nil
	})
}

func sha3file(path string) (string, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		err = errors.Wrap(err, "failed to read .sol file")
		return "", err
	}

	hashBytes := crypto.Keccak256(contents)
	return hex.EncodeToString(hashBytes), nil
}
