package main

import (
	"fmt"
	"path/filepath"

	cli "github.com/jawher/mow.cli"
	log "github.com/xlab/suplog"
)

func onBuild(cmd *cli.Cmd) {
	cmd.Action = func() {
		solc := getCompiler()

		solSourceFullPath, _ := filepath.Abs(*solSource)
		contract := getCompiledContract(solc, *contractName, solSourceFullPath, false)

		if !*noCache {
			cacheLog := log.WithField("path", *buildCacheDir)
			cache, err := NewBuildCache(*buildCacheDir)
			if err != nil {
				cacheLog.WithError(err).Warningln("failed to use build cache dir")
			} else if err := cache.StoreContract(solSourceFullPath, contract); err != nil {
				cacheLog.WithError(err).Warningln("failed to store contract code in build cache")
			}
		}

		fmt.Println(contract.Bin)
	}
}
