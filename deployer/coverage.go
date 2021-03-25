package deployer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/InjectiveLabs/etherman/sol"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ctypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/tools/cover"
)

type CoverageDataCollector interface {
	LoadContract(contract *sol.Contract) error
	AddStatement(contractName string, start, end, file uint64) error
	CollectCoverageEvent(contractName string, coverageEventABI abi.Event, log *ctypes.Log) error
	CollectCoverageRevert(contractName string, err error) error
	ReportTextSummary(out io.Writer, filterNames ...string) error
	ReportTextCoverfile(out io.Writer, filterNames ...string) error
	ReportHTML(out io.Writer, filterNames ...string) error
}

type CoverageMode string

const (
	CoverageModeSet     CoverageMode = "set"
	CoverageModeCount   CoverageMode = "count"
	CoverageModeDefault CoverageMode = CoverageModeCount
)

func NewCoverageDataCollector(mode CoverageMode) CoverageDataCollector {
	return &coverageDataCollector{
		mux:          new(sync.RWMutex),
		paths:        make(map[string][]string),
		srcFiles:     make(map[string][]*fileMapping),
		statements:   make(map[statementDescriptor]int),
		coverageMode: mode,
	}
}

type coverageDataCollector struct {
	mux          *sync.RWMutex
	paths        map[string][]string
	srcFiles     map[string][]*fileMapping
	statements   map[statementDescriptor]int
	coverageMode CoverageMode
}

type coverageEvent struct {
	Start uint64
	End   uint64
	File  uint64
}

type statementDescriptor struct {
	SrcLocation        string
	ContractName       string
	LineStart, LineEnd int
	ColStart, ColEnd   int
}

func (s statementDescriptor) String() string {
	return fmt.Sprintf("%s:%d.%d,%d.%d", s.SrcLocation, s.LineStart, s.ColStart, s.LineEnd, s.ColEnd)
}

func (c *coverageDataCollector) LoadContract(contract *sol.Contract) error {
	if !contract.Coverage {
		return ErrNoCoverage
	}

	if len(contract.AllPaths) == 0 {
		return errors.New("contract doesn't have solity files paths")
	}

	var openErr error

	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.paths[contract.Name]; ok {
		return nil
	}

	c.srcFiles[contract.Name] = make([]*fileMapping, len(contract.AllPaths))

	for idx, solPath := range contract.AllPaths {
		lines, err := readLines(solPath)
		if err != nil {
			openErr = multierror.Append(openErr, err)
			continue
		}

		pos := 0
		mapping := new(fileMapping)
		for lineNum, line := range lines {
			mapping.AddLine(lineNum, pos, pos+len(line)+1)
			pos = pos + len(line) + 1
		}

		c.srcFiles[contract.Name][idx] = mapping
	}

	if openErr != nil {
		return openErr
	}

	c.paths[contract.Name] = contract.AllPaths

	return nil
}

func (c *coverageDataCollector) AddStatement(contractName string, start, end, file uint64) (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.paths[contractName]; !ok {
		err = errors.Errorf("contract sources not found: %s", contractName)
		return err
	}

	statement := statementDescriptor{
		SrcLocation:  c.paths[contractName][file],
		ContractName: contractName,
	}
	statement.LineStart, statement.ColStart = c.srcFiles[contractName][int(file)].PosToLine(int(start))
	statement.LineEnd, statement.ColEnd = c.srcFiles[contractName][int(file)].PosToLine(int(start + end))

	if _, existing := c.statements[statement]; !existing {
		c.statements[statement] = 0
	}

	return nil
}

func (c *coverageDataCollector) CollectCoverageEvent(contractName string, coverageEventABI abi.Event, log *ctypes.Log) error {
	values, err := coverageEventABI.Inputs.Unpack(log.Data)
	if err != nil {
		err = errors.Wrap(err, "coverage event ABI unpack error")
		return err
	}

	var ev coverageEvent
	if err := coverageEventABI.Inputs.Copy(&ev, values); err != nil {
		err = errors.Wrap(err, "coverage event read error")
		return err
	}

	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.paths[contractName]; !ok {
		err = errors.Errorf("contract sources not found: %s", contractName)
		return err
	}

	statement := statementDescriptor{
		SrcLocation:  c.paths[contractName][ev.File],
		ContractName: contractName,
	}
	statement.LineStart, statement.ColStart = c.srcFiles[contractName][int(ev.File)].PosToLine(int(ev.Start))
	statement.LineEnd, statement.ColEnd = c.srcFiles[contractName][int(ev.File)].PosToLine(int(ev.Start + ev.End))

	c.statements[statement] += 1

	return nil
}

const coverageRevertTag = " @coverage"

func hasCoverageReport(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), coverageRevertTag)
}

func trimCoverageReport(err error) error {
	idx := strings.LastIndex(err.Error(), coverageRevertTag)
	if idx == -1 {
		return err
	}

	return errors.New(err.Error()[:idx])
}

func (c *coverageDataCollector) CollectCoverageRevert(contractName string, err error) error {
	idx := strings.LastIndex(err.Error(), coverageRevertTag)
	if idx == -1 {
		return errors.New("not a @coverage revert message")
	} else {
		idx += len(coverageRevertTag) + 1 // @coverage,1,2,3
	}

	locationParts := strings.Split(err.Error()[idx:], ",")
	if len(locationParts) != 3 {
		return errors.New("@coverage revert message contains wrong location")
	}

	var (
		start, _ = strconv.Atoi(locationParts[0])
		end, _   = strconv.Atoi(locationParts[1])
		file, _  = strconv.Atoi(locationParts[2])
	)
	if start < 0 || end < 0 || file < 0 {
		return nil
	}

	c.mux.Lock()
	defer c.mux.Unlock()

	if _, ok := c.paths[contractName]; !ok {
		err = errors.Errorf("contract sources not found: %s", contractName)
		return err
	}

	statement := statementDescriptor{
		SrcLocation:  c.paths[contractName][file],
		ContractName: contractName,
	}
	statement.LineStart, statement.ColStart = c.srcFiles[contractName][file].PosToLine(start)
	statement.LineEnd, statement.ColEnd = c.srcFiles[contractName][file].PosToLine(start + end)

	c.statements[statement] += 1

	return nil
}

func (c *coverageDataCollector) ReportTextSummary(out io.Writer, filterNames ...string) (err error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return nil
}

func (c *coverageDataCollector) ReportTextCoverfile(out io.Writer, filterNames ...string) (err error) {
	c.mux.RLock()
	defer c.mux.RUnlock()

	filters := make(map[string]struct{}, len(filterNames))
	for _, name := range filterNames {
		filters[name] = struct{}{}
	}

	_, writeErr := fmt.Fprintf(out, "mode: %s\n", c.coverageMode)
	if writeErr != nil {
		err = multierror.Append(err, writeErr)
	}

	for desc, count := range c.statements {
		if len(filters) > 0 {
			if _, ok := filters[desc.ContractName]; !ok {
				continue
			}
		}

		if c.coverageMode == CoverageModeCount {
			_, writeErr = fmt.Fprintf(out, "%s 1 %d\n", desc.String(), count)
		} else if c.coverageMode == CoverageModeSet {
			var set = 0
			if count > 0 {
				set = 1
			}

			_, writeErr = fmt.Fprintf(out, "%s 1 %d\n", desc.String(), set)
		} else {
			return errors.Errorf("unsupported coverageMode: %s", c.coverageMode)
		}

		if writeErr != nil {
			err = multierror.Append(err, writeErr)
		}
	}

	return nil
}

func (c *coverageDataCollector) ReportHTML(out io.Writer, filterNames ...string) error {
	c.mux.RLock()
	defer c.mux.RUnlock()

	filters := make(map[string]struct{}, len(filterNames))
	for _, name := range filterNames {
		filters[name] = struct{}{}
	}

	profiles := make(map[string]*cover.Profile, len(c.statements))
	for desc, count := range c.statements {
		if len(filters) > 0 {
			if _, ok := filters[desc.ContractName]; !ok {
				continue
			}
		}

		if c.coverageMode == CoverageModeSet {
			if count > 0 {
				count = 1
			}
		} else if c.coverageMode != CoverageModeCount {
			return errors.Errorf("unsupported coverageMode: %s", c.coverageMode)
		}

		if profiles[desc.SrcLocation] == nil {
			profiles[desc.SrcLocation] = &cover.Profile{
				FileName: desc.SrcLocation,
				Mode:     string(c.coverageMode),
			}
		}
		profiles[desc.SrcLocation].Blocks = append(profiles[desc.SrcLocation].Blocks, cover.ProfileBlock{
			StartLine: desc.LineStart,
			StartCol:  desc.ColStart,
			EndLine:   desc.LineEnd,
			EndCol:    desc.ColEnd,
			NumStmt:   1,
			Count:     count,
		})
	}

	return c.htmlOutput(profiles, out)
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

type newlineBoundary [2]int

type fileMapping struct {
	initOnce          sync.Once
	newlineBoundaries []newlineBoundary
}

func (f *fileMapping) AddLine(n, start, end int) {
	f.newlineBoundaries = append(f.newlineBoundaries, newlineBoundary{start, end})
}

func (f *fileMapping) PosToLine(pos int) (line, column int) {
	for line, boundary := range f.newlineBoundaries {
		if pos >= boundary[0] && pos <= boundary[1] {
			return line + 1, pos - boundary[0] + 1
		}
	}

	return -1, -1
}
