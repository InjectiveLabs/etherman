// Package sol provides a convenient interface for calling the 'solc' Solidity Compiler from Go.
package sol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
	log "github.com/xlab/suplog"
)

type Contract struct {
	Name            string
	SourcePath      string
	AllPaths        []string
	CompilerVersion string
	Address         common.Address
	Coverage        bool
	Statements      [][]int

	ABI []byte
	Bin string
}

type Compiler interface {
	SetAllowPaths(paths []string) Compiler
	Compile(prefix, path string, optimize int) (map[string]*Contract, error)
	CompileWithCoverage(prefix, path string) (map[string]*Contract, error)
}

func NewSolCompiler(solcPath string) (Compiler, error) {
	s := &solCompiler{
		solcPath: solcPath,
	}
	if err := s.verify(); err != nil {
		return nil, err
	}
	return s, nil
}

type solCompiler struct {
	solcPath   string
	allowPaths []string
}

func (s *solCompiler) verify() error {
	out, err := exec.Command(s.solcPath, "--version").CombinedOutput()
	if err != nil {
		err = fmt.Errorf("solc verify: failed to exec solc: %v", err)
		return err
	}
	hasPrefix := strings.HasPrefix(string(out), "solc, the solidity compiler")
	if !hasPrefix {
		err := fmt.Errorf("solc verify: executable output was unexpected (output: %s)", out)
		return err
	}
	return nil
}

func (s *solCompiler) SetAllowPaths(paths []string) Compiler {
	s.allowPaths = paths
	return s
}

type solcContract struct {
	ABI json.RawMessage `json:"abi"`
	Bin string          `json:"bin"`
}

type solcSource struct {
	AST json.RawMessage `json:"AST,omitempty"`
}

type solcOutput struct {
	Contracts  map[string]solcContract `json:"contracts"`
	Sources    map[string]solcSource   `json:"sources,omitempty"`
	SourceList []string                `json:"sourceList,omitempty"`
	Version    string                  `json:"version"`
}

func (s *solCompiler) Compile(prefix, path string, optimize int) (map[string]*Contract, error) {
	args := []string{s.solcPath}
	if len(s.allowPaths) > 0 {
		args = append(args, "--allow-paths", strings.Join(s.allowPaths, ","))
	}
	args = append(args, "--combined-json", "bin,abi,ast", filepath.Join(prefix, path))
	if optimize > 0 {
		args = append(args, "--optimize", fmt.Sprintf("--optimize-runs=%d", optimize))
	}
	cmd := exec.Cmd{
		Path:   s.solcPath,
		Args:   args,
		Dir:    prefix,
		Stderr: os.Stderr,
	}

	log.Infoln("Running solc compiler:", cmd.String())

	out, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("solc: failed to compile contract: %v", err)
		return nil, err
	}

	var result solcOutput
	if err := json.Unmarshal(out, &result); err != nil {
		err = fmt.Errorf("solc: failed to unmarshal Solc output: %v", err)
		return nil, err
	}

	if len(result.Contracts) == 0 {
		err := errors.New("solc: no contracts compiled")
		return nil, err
	} else if len(result.Sources) == 0 {
		err := errors.New("solc: no source paths collected")
		return nil, err
	}

	contractPathsByName := make(map[string]string, len(result.SourceList))
	contractNamesOrdered := make([]string, len(result.SourceList))
	contractFilePaths := make([]string, 0, len(result.SourceList))

	for id := range result.Contracts {
		name, sourcePath, err := idToNameAndSourcePath(id)
		if err != nil {
			return nil, err
		}

		contractPathsByName[name] = sourcePath
	}
	for name, sourcePath := range contractPathsByName {
		for idx, src := range result.SourceList {
			if src == sourcePath {
				contractNamesOrdered[idx] = name
				break
			}
		}
	}

	seenPaths := make(map[string]struct{}, len(contractPathsByName))

	for _, contractName := range contractNamesOrdered {
		filePath := contractPathsByName[contractName]
		if _, ok := seenPaths[filePath]; ok {
			continue
		} else {
			seenPaths[filePath] = struct{}{}
		}
		contractFilePaths = append(contractFilePaths, filePath)
	}

	contracts := make(map[string]*Contract, len(result.Contracts))
	for id, c := range result.Contracts {
		name, sourcePath, err := idToNameAndSourcePath(id)
		if err != nil {
			return nil, err
		}

		contracts[name] = &Contract{
			Name:            name,
			SourcePath:      sourcePath,
			AllPaths:        contractFilePaths,
			CompilerVersion: result.Version,
			Coverage:        false,

			ABI: []byte(c.ABI),
			Bin: c.Bin,
		}
	}

	return contracts, nil
}

func (s *solCompiler) CompileWithCoverage(prefix, path string) (map[string]*Contract, error) {
	args := []string{s.solcPath}
	if len(s.allowPaths) > 0 {
		args = append(args, "--allow-paths", strings.Join(s.allowPaths, ","))
	}

	args = append(args, "--optimize", "--combined-json", "ast", filepath.Join(prefix, path))

	cmd := exec.Cmd{
		Path:   s.solcPath,
		Args:   args,
		Dir:    prefix,
		Stderr: os.Stderr,
	}

	log.Infoln("Running solc compiler:", cmd.String())

	out, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("solc: failed to compile contract: %v", err)
		return nil, err
	}

	var result solcOutput
	if err := json.Unmarshal(out, &result); err != nil {
		err = fmt.Errorf("solc: failed to unmarshal Solc output: %v", err)
		return nil, err
	}
	if len(result.Contracts) == 0 {
		err := errors.New("solc: no contracts compiled")
		return nil, err
	} else if len(result.Sources) == 0 {
		err := errors.New("solc: source paths collected")
		return nil, err
	}

	contractPathsByName := make(map[string]string, len(result.SourceList))
	contractNamesOrdered := make([]string, len(result.SourceList))
	contractFilePaths := make([]string, 0, len(result.SourceList))

	for id := range result.Contracts {
		name, sourcePath, err := idToNameAndSourcePath(id)
		if err != nil {
			return nil, err
		}

		contractPathsByName[name] = sourcePath
	}

	for name, sourcePath := range contractPathsByName {
		for idx, src := range result.SourceList {
			if src == sourcePath {
				contractNamesOrdered[idx] = name
				break
			}
		}
	}

	seenPaths := make(map[string]struct{}, len(contractPathsByName))

	contractStatements := make([][]int, 0, len(contractFilePaths))
	for fileIdx, contractName := range contractNamesOrdered {
		filePath := contractPathsByName[contractName]

		if _, ok := seenPaths[filePath]; ok {
			err := errors.Errorf("multiple contracts in the same file is a big no-no. Please refactor %s", filePath)
			return nil, err
		} else {
			seenPaths[filePath] = struct{}{}
		}

		source := result.Sources[filePath]

		modifiedAST, statements, err := addCoverageMarkers(fileIdx, contractName, source.AST)
		if err != nil {
			err = errors.Wrapf(err, "failed to orchestrate %s source with coverage markers", filePath)
			return nil, err
		}

		contractFilePaths = append(contractFilePaths, filePath)
		contractStatements = append(contractStatements, statements...)

		escapedPath := strings.Replace(filePath, ".", "\\.", -1)
		out, err = sjson.SetBytes(out, fmt.Sprintf("sources.%s.AST", escapedPath), modifiedAST)
		if err != nil {
			err = errors.Wrap(err, "failed to update solc output with added coverage")
			return nil, err
		}
	}

	tmp, err := os.CreateTemp("", "*_sol_coverage.json")
	if err != nil {
		err = errors.Wrap(err, "failed to open temp file for orchestrated AST output")
		return nil, err
	}

	// fmt.Println("Staging coverage into:", tmp.Name())
	if _, err := io.Copy(tmp, bytes.NewReader(out)); err != nil {
		err = errors.Wrap(err, "failed to write temp file with orchestrated AST output")
		return nil, err
	}

	_ = tmp.Close()

	defer func() {
		_ = os.Remove(tmp.Name())
	}()

	// now just re-import the patched AST

	args = []string{s.solcPath}
	args = append(args, "--import-ast", "--optimize", "--combined-json", "bin,abi", tmp.Name())
	errOut := new(bytes.Buffer)
	finalCmd := exec.Cmd{
		Path:   s.solcPath,
		Args:   args,
		Dir:    prefix,
		Stderr: errOut,
	}

	log.Infoln("Running solc compiler:", finalCmd.String())

	finalOutput, err := finalCmd.Output()
	if err != nil {
		_, _ = io.Copy(os.Stderr, errOut)
		err = fmt.Errorf("solc: failed to compile contract: %v", err)
		return nil, err
	}

	var finalResult solcOutput
	if err := json.Unmarshal(finalOutput, &finalResult); err != nil {
		err = fmt.Errorf("solc: failed to unmarshal Solc output: %v", err)
		return nil, err
	}

	if len(finalResult.Contracts) == 0 {
		err := errors.New("solc: no contracts compiled")
		return nil, err
	}

	contracts := make(map[string]*Contract, len(finalResult.Contracts))
	for id, c := range finalResult.Contracts {
		name, sourcePath, err := idToNameAndSourcePath(id)
		if err != nil {
			return nil, err
		}

		contracts[name] = &Contract{
			Name:            name,
			SourcePath:      sourcePath,
			AllPaths:        contractFilePaths,
			CompilerVersion: finalResult.Version,
			Coverage:        true,
			Statements:      contractStatements,

			ABI: []byte(c.ABI),
			Bin: c.Bin,
		}
	}

	return contracts, nil
}

func idToNameAndSourcePath(id string) (name, sourcePath string, err error) {
	idParts := strings.Split(id, ":")
	if len(idParts) == 1 {
		err = errors.Errorf("solc: found an unnamed contract in output: %s", id)
		return
	}

	name = idParts[len(idParts)-1]
	sourcePath = idParts[0]

	return name, sourcePath, nil
}

func WhichSolc() (string, error) {
	out, err := exec.Command("which", "solc").Output()
	if err != nil {
		return "", errors.New("solc executable file not found in $PATH")
	}
	return string(bytes.TrimSpace(out)), nil
}
