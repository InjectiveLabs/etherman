## evm-deploy-contract

A tool for contract deployment and testing.

### Usage

Install `solc` first, the executable will be located automatically.

```
$ evm-deploy-contract --help

Usage: evm-deploy-contract [OPTIONS] COMMAND [arg...]

Deploys arbitrary contract on an arbitrary EVM. Requires solc 0.6.x or later.

Options:
      --solc-path         Set path solc executable. Found using 'which' otherwise (env $DEPLOYER_SOLC_PATH)
  -N, --name              Specify contract name to use. (env $DEPLOYER_CONTRACT_NAME) (default "Counter")
  -S, --source            Set path for .sol source file of the contract. (env $DEPLOYER_SOL_SOURCE_FILE) (default "contracts/Counter.sol")
  -E, --endpoint          Specify the JSON-RPC endpoint for accessing Ethereum node (env $DEPLOYER_RPC_URI) (default "http://localhost:8545")
  -G, --gas-price         Override estimated gas price with this option. (env $DEPLOYER_TX_GAS_PRICE) (default 50)
  -L, --gas-limit         Set the maximum gas for tx. (env $DEPLOYER_TX_GAS_LIMIT) (default 5000000)
      --cache-dir         Set cache dir for build artifacts. (env $DEPLOYER_CACHE_DIR) (default "build/")
      --no-cache          Disables build cache completely. (env $DEPLOYER_DISABLE_CACHE)
      --cover             Enables code coverage orchestration (env $DEPLOYER_ENABLE_COVERAGE)
      --keystore-dir      Specify Ethereum keystore dir (Geth or Clef) prefix. (env $DEPLOYER_KEYSTORE_DIR)
  -F, --from              Specify the from address. If specified, must exist in keystore, ledger or match the privkey. (env $DEPLOYER_FROM)
      --from-passphrase   Passphrase to unlock the private key from armor, if empty then stdin is used. (env $DEPLOYER_FROM_PASSPHRASE)
  -P, --from-pk           Provide a raw Ethereum private key of the validator in hex. (env $DEPLOYER_FROM_PK)
      --ledger            Use the Ethereum app on hardware ledger to sign transactions. (env $DEPLOYER_USE_LEDGER)

Commands:
  build                   Builds given contract and cached build artefacts. Optional step.
  deploy                  Deploys given contract on the EVM chain. Caches build artefacts.
  tx                      Creates a transaction for particular contract method. Uses build cache.
  call                    Calls method of a particular contract. Uses build cache.
  logs                    Loads logs of a particular event from contract.

Run 'evm-deploy-contract COMMAND --help' for more information on a command.

```

### Deploying

```
$ evm-deploy-contract deploy --help

Usage: evm-deploy-contract deploy [--bytecode | --await] [ARGS...]

Deploys given contract on the EVM chain. Caches build artefacts.

Arguments:
  ARGS             Contract constructor's arguments. Will be ABI-encoded.

Options:
      --bytecode   Produce hex-encoded contract bytecode only. Do not interact with RPC.
      --await      Await transaction confirmation from the RPC. (default true)
```

**Example**

```
$ evm-deploy-contract -E http://localhost:1317 -P 59F455CBF7B02A2C1F6B55B4D8D8FEF21BCD530457A9570999FB1C12C82F5201 -G 0 deploy
```

```
$ evm-deploy-contract --source contracts/Counter.sol deploy --bytecode
```

### Method transact

```
$ evm-deploy-contract tx --help

Usage: evm-deploy-contract tx [--await] ADDRESS METHOD [ARGS...]

Creates a transaction for particular contract method. Uses build cache.

Arguments:
  ADDRESS          Contract address to interact with.
  METHOD           Contract method to transact.
  ARGS             Method transaction arguments. Will be ABI-encoded.

Options:
      --bytecode   Produce hex-encoded ABI-packed params bytecode only. Do not interact with RPC.
      --await      Await transaction confirmation from the RPC. (default true)
```

**Example**

```
$ evm-deploy-contract -E http://localhost:1317 -P 1F2FAB11FA77AE1110D9E9AF59191C656B8BA1093F1480F99486F635E38597CC -G 0 \
    tx 0x33832d3A5e359A0689088c832755461dDaD5d41B add
```

```
$ evm-deploy-contract -E http://localhost:1317 -P 1F2FAB11FA77AE1110D9E9AF59191C656B8BA1093F1480F99486F635E38597CC -G 0 \
    tx --await=false 0x33832d3A5e359A0689088c832755461dDaD5d41B addValue 10
```

### Read logs

```
$ evm-deploy-contract logs --help

Usage: evm-deploy-contract logs ADDRESS TX_HASH EVENT_NAME

Loads logs of a particular event from contract.

Arguments:
  ADDRESS      Contract address to interact with.
  TX_HASH      Transaction hash to find receipt.
  EVENT_NAME   Contract event to find in the logs.
```

**Example**

```
evm-deploy-contract -E http://localhost:1317 logs 0x33832d3A5e359A0689088c832755461dDaD5d41B 0x8d2a06a2811cc4be16536c54e693ef1c268f8d04956fa0899e18372f6201fbe9 Increment
```

### License

[MIT](/LICENSE)
