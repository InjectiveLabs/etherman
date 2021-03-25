package deployer

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/etherman/sol"
)

var (
	ErrCompilationFailed = errors.New("failed to compile contract code")
)

func (d *deployer) Build(
	ctx context.Context,
	solSource string,
	contractName string,
) (*sol.Contract, error) {
	solSourceFullPath, _ := filepath.Abs(solSource)
	contract := d.getCompiledContract(contractName, solSourceFullPath)
	if contract == nil {
		log.Errorln("contract compilation failed, check logs")
		return nil, ErrCompilationFailed
	}

	return contract, nil
}
