package deployer

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/xlab/suplog"

	"github.com/InjectiveLabs/evm-deploy-contract/sol"
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
	contract := d.getCompiledContract(contractName, solSourceFullPath, false)
	if contract == nil {
		return nil, ErrCompilationFailed
	}

	if !d.options.NoCache {
		cacheLog := log.WithField("cache_dir", d.options.BuildCacheDir)
		cache, err := NewBuildCache(d.options.BuildCacheDir)
		if err != nil {
			cacheLog.WithError(err).Warningln("failed to use build cache dir")
		} else if err := cache.StoreContract(solSourceFullPath, contract); err != nil {
			cacheLog.WithError(err).Warningln("failed to store contract code in build cache")
		}
	}

	return contract, nil
}
